package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/relay/internal/capture"
	"ddbot/relay/internal/framing"
	"ddbot/relay/internal/link"
	"ddbot/relay/internal/packets"
)

type memoryRecorder struct {
	mu      sync.Mutex
	records []capture.Record
}

func (r *memoryRecorder) Write(record capture.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	record.Data = bytes.Clone(record.Data)
	r.records = append(r.records, record)

	return nil
}

func TestRemoteBotExecutesRelayUploadAndPreservesCipher(t *testing.T) {
	if testing.Short() {
		t.Skip("requires building and starting the bot application")
	}

	var networkMu sync.Mutex
	var offline bool
	var sockets []net.Conn
	cc, ctx := startBot(t, "''", func(ctx context.Context, address string) (net.Conn, error) {
		networkMu.Lock()
		blocked := offline
		networkMu.Unlock()
		if blocked {
			return nil, io.ErrClosedPipe
		}

		connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
		if err != nil {
			return nil, err
		}

		networkMu.Lock()
		defer networkMu.Unlock()
		if offline {
			return nil, errors.Join(io.ErrClosedPipe, connection.Close())
		}

		sockets = append(sockets, connection)

		return connection, nil
	})
	client, proxyClient := net.Pipe()
	server, proxyServer := net.Pipe()
	defer client.Close()
	defer server.Close()
	recorder := &memoryRecorder{}
	var automation *link.Link
	connection := NewConnection(1, proxyClient, proxyServer, recorder, nil, packets.NewDecoder(), func(checkpoint func(context.Context) (link.Checkpoint, error)) Automation {
		automation = link.New(pb.NewBotClient(cc), "synthetic-test-api-key", &pb.OpenSession{
			ApiVersion:      contract.APIVersion,
			CodecRevision:   contract.CodecRevision,
			RelayInstanceId: "test-relay",
			GameSessionId:   "test-game",
			LocalStream:     1,
		}, "", slog.New(slog.NewJSONHandler(io.Discard, nil)), checkpoint)

		return automation
	})
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- connection.Run(runCtx) }()
	defer func() {
		stop()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	if err := client.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}

	if err := server.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}

	send := func(from, to net.Conn, data []byte) {
		t.Helper()
		written := make(chan error, 1)
		go func() { _, err := from.Write(data); written <- err }()
		received := make([]byte, len(data))
		if _, err := io.ReadFull(to, received); err != nil {
			t.Fatal(err)
		}

		if err := <-written; err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(received, data) {
			t.Fatalf("forwarding changed bytes")
		}
	}
	key := [8]byte{3, 19, 47, 61, 89, 103, 131, 157}
	first := encrypted(t, &key, framing.Frame{Header: framing.Header{Code: 4}})
	second := encrypted(t, &key, framing.Frame{Header: framing.Header{Code: 6}})
	send(client, server, append(first, second...))
	serverKey := [8]byte{11, 23, 31, 43, 59, 71, 83, 97}
	login := encrypted(t, &serverKey, framing.Frame{Header: framing.Header{Code: 1, ClientID: 123}, Body: loginBody()})
	keepalive := encrypted(t, &serverKey, framing.Frame{Header: framing.Header{Code: 4}, Body: []byte{0, 0, 0, 2}})
	send(server, client, append(login, keepalive...))
	deadline := time.Now().Add(5 * time.Second)

	for {
		result, err := connection.Request(ctx, "inspect-"+time.Now().Format("150405.000000000"), "eval", `return player.state()`)
		if err == nil && result.Error == nil {
			value, err := contract.DecodeValue(result.Value)
			if err != nil {
				t.Fatal(err)
			}

			encoded, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}

			if string(encoded) == `{"bag_revisions":{},"buffs_known":false,"id":123,"inventory_revision":0,"inventory_uncertain":false}` {
				break
			}
		}

		if time.Now().After(deadline) {
			t.Fatalf("login projection not ready: %v %v; link=%+v", result, err, automation.State())
		}

		time.Sleep(10 * time.Millisecond)
	}

	source := `return {api_version=1,on_packet=function(p) seen=(seen or 0)+1 end}`
	loaded, err := connection.Request(ctx, "load", "load", source)
	if err != nil || loaded.Error != nil {
		t.Fatalf("load %v %v", loaded, err)
	}

	result, err := connection.Request(ctx, "fire", "eval", `return room.start()`)
	if err != nil || result.Error != nil {
		t.Fatalf("eval %v %v", result, err)
	}

	injected := make([]byte, 24)
	if _, err := io.ReadFull(server, injected); err != nil {
		t.Fatal(err)
	}

	third := encrypted(t, &key, framing.Frame{Header: framing.Header{Code: 91}, Body: []byte{12, 0}})
	written := make(chan error, 1)
	go func() { _, err := client.Write(third); written <- err }()
	translated := make([]byte, len(third))
	if _, err := io.ReadFull(server, translated); err != nil {
		t.Fatal(err)
	}

	if err := <-written; err != nil {
		t.Fatal(err)
	}

	wire := append(append(append(append([]byte{}, first...), second...), injected...), translated...)
	frames := framing.NewEncryptedFramer().Push(wire)
	if len(frames) != 4 || frames[2].Frame.Header.Code != 94 || !bytes.Equal(frames[2].Frame.Body, []byte{0, 0, 0, 7}) || frames[3].Frame.Header.Code != 91 || !bytes.Equal(frames[3].Frame.Body, []byte{12, 0}) {
		t.Fatalf("cipher continuity: %#v", frames)
	}

	if _, err := connection.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}

	before, err := connection.Request(ctx, "before-loss", "eval", `return seen`)
	if err != nil || before.Error != nil {
		t.Fatalf("before loss %v %v", before, err)
	}

	count, err := contract.DecodeValue(before.Value)
	if err != nil {
		t.Fatal(err)
	}

	networkMu.Lock()
	offline = true
	closing := sockets
	sockets = nil
	networkMu.Unlock()

	for _, socket := range closing {
		if err := socket.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Fatal(err)
		}
	}

	if len(closing) == 0 {
		t.Fatal("no gRPC sockets were opened")
	}

	deadline = time.Now().Add(5 * time.Second)

	for automation.State().Attached {
		if time.Now().After(deadline) {
			t.Fatalf("link did not detach: %+v", automation.State())
		}

		time.Sleep(time.Millisecond)
	}

	fourth := encrypted(t, &key, framing.Frame{Header: framing.Header{Code: 6}})
	go func() { _, err := client.Write(fourth); written <- err }()
	afterLoss := make([]byte, len(fourth))
	if _, err := io.ReadFull(server, afterLoss); err != nil {
		t.Fatal(err)
	}

	if err := <-written; err != nil {
		t.Fatal(err)
	}

	networkMu.Lock()
	offline = false
	networkMu.Unlock()
	deadline = time.Now().Add(10 * time.Second)

	for !automation.State().Attached || automation.State().Generation != 2 {
		if time.Now().After(deadline) {
			t.Fatalf("resume failed %+v", automation.State())
		}

		time.Sleep(10 * time.Millisecond)
	}

	after, err := connection.Request(ctx, "after-loss", "eval", `return seen`)
	if err != nil || after.Error != nil {
		t.Fatalf("after loss %v %v", after, err)
	}

	afterCount, err := contract.DecodeValue(after.Value)
	if err != nil || afterCount != count.(float64)+1 {
		t.Fatalf("replay callbacks before=%v after=%v error=%v", count, afterCount, err)
	}

	allFrames := framing.NewEncryptedFramer().Push(append(wire, afterLoss...))
	if len(allFrames) != 5 || allFrames[4].Frame.Header.Code != 6 {
		t.Fatalf("forwarding through outage %#v", allFrames)
	}

	again, err := connection.Request(ctx, "fire", "eval", `return room.start()`)
	if err != nil || again.Error != nil || len(again.ActionIds) != 1 || again.ActionIds[0] != result.ActionIds[0] {
		t.Fatalf("duplicate evaluation %v %v", again, err)
	}

	if err := server.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}

	n, err := server.Read(make([]byte, 1))
	if timeout, ok := err.(net.Error); n != 0 || !ok || !timeout.Timeout() {
		t.Fatalf("duplicate command reached game: %d %v", n, err)
	}
}

func encrypted(t *testing.T, key *[8]byte, frame framing.Frame) []byte {
	t.Helper()
	plain, err := framing.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}

	wire := make([]byte, len(plain))

	for i, b := range plain {
		if i == 0 {
			wire[i] = b ^ key[0]
			continue
		}

		slot := i % 8
		key[slot] = (key[slot] + wire[i-1]) ^ byte(i)
		wire[i] = (b ^ key[slot]) + wire[i-1]
	}

	return wire
}
