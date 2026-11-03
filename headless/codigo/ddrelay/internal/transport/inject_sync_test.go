package transport

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"

	"ddbot/relay/internal/framing"
	"ddbot/relay/internal/link"
	"ddbot/relay/internal/packets"
)

func TestInjectionKeepsClientStreamDecodable(t *testing.T) {
	client, proxyClient := net.Pipe()
	server, proxyServer := net.Pipe()
	defer client.Close()
	defer server.Close()

	automation := &fakeAutomation{done: make(chan struct{})}
	connection := NewConnection(1, proxyClient, proxyServer, &memoryRecorder{}, nil, packets.NewDecoder(), func(func(context.Context) (link.Checkpoint, error)) Automation {
		return automation
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- connection.Run(ctx) }()

	key := [8]byte{3, 19, 47, 61, 89, 103, 131, 157}
	first := encrypted(t, &key, framing.Frame{Header: framing.Header{Code: 4}})
	second := encrypted(t, &key, framing.Frame{Header: framing.Header{Code: 4}})
	third := encrypted(t, &key, framing.Frame{Header: framing.Header{Code: 4}})
	injected := framing.Frame{Header: framing.Header{Code: 4, Parameter1: 7}}
	plain, err := framing.Marshal(injected)
	if err != nil {
		t.Fatal(err)
	}

	write := func(data []byte) error {
		_, err := client.Write(data)

		return err
	}

	read := func(length int) []byte {
		t.Helper()
		buffer := make([]byte, length)
		if _, err := io.ReadFull(server, buffer); err != nil {
			t.Fatal(err)
		}

		return buffer
	}

	var stream []byte
	pending := make(chan error, 1)
	go func() { pending <- write(append(bytes.Clone(first), second...)) }()
	stream = append(stream, read(len(first)+len(second))...)
	if err := <-pending; err != nil {
		t.Fatal(err)
	}

	injectedWrite := make(chan struct {
		written int
		err     error
	}, 1)
	go func() {
		written, err := connection.Inject(ctx, injected)
		injectedWrite <- struct {
			written int
			err     error
		}{written, err}
	}()

	stream = append(stream, read(len(plain))...)
	if result := <-injectedWrite; result.err != nil || result.written != len(plain) {
		t.Fatalf("injection wrote %d bytes with error %v, want %d", result.written, result.err, len(plain))
	}

	pending = make(chan error, 1)
	go func() { pending <- write(third) }()
	stream = append(stream, read(len(third))...)
	if err := <-pending; err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		info, err := connection.Inspect(ctx)
		if err != nil {
			break
		}

		if info.EventSequence >= 4 {
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("injected and subsequent client frames were not observed")
		}

		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	if automation.invalid != "" {
		t.Fatalf("injection invalidated the stream: %s", automation.invalid)
	}

	segments := framing.NewEncryptedFramer().Push(stream)
	if len(segments) != 4 {
		t.Fatalf("server view decoded %d segments, want 4", len(segments))
	}

	for index, segment := range segments {
		if segment.Kind != framing.FrameSegment {
			t.Fatalf("server view segment %d is not a frame: %s", index, segment.Note)
		}
	}

	if !bytes.Equal(segments[2].Frame.Body, nil) || segments[2].Frame.Header.Parameter1 != 7 {
		t.Fatalf("server view lost the injected frame fields: %+v", segments[2].Frame.Header)
	}
}
