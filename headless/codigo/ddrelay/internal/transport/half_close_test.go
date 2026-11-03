package transport

import (
	"context"
	"net"
	"testing"
	"time"

	"ddbot/relay/internal/link"
	"ddbot/relay/internal/packets"
)

type halfClosePeer struct {
	net.Conn
	writeClosed chan struct{}
}

func (c *halfClosePeer) CloseWrite() error { close(c.writeClosed); return nil }

func TestConnectionWaitsForBothTCPReadSides(t *testing.T) {
	client, clientProxy := net.Pipe()
	server, serverProxy := net.Pipe()
	defer client.Close()
	defer server.Close()
	downstream := &halfClosePeer{Conn: clientProxy, writeClosed: make(chan struct{})}
	upstream := &halfClosePeer{Conn: serverProxy, writeClosed: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	connection := NewConnection(1, downstream, upstream, &memoryRecorder{}, nil, packets.NewDecoder(), func(func(context.Context) (link.Checkpoint, error)) Automation {
		return &fakeAutomation{done: make(chan struct{})}
	})
	done := make(chan error, 1)
	go func() { done <- connection.Run(ctx) }()
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}

	select {
	case <-downstream.writeClosed:
	case <-ctx.Done():
		t.Fatal("server EOF was not half-closed toward client")
	}

	if _, err := connection.Inspect(ctx); err != nil {
		t.Fatal("connection ended before client EOF:", err)
	}

	if err := client.Close(); err != nil {
		t.Fatal(err)
	}

	if err := <-done; err != nil {
		t.Fatal(err)
	}

	select {
	case <-upstream.writeClosed:
	default:
		t.Fatal("client EOF was not half-closed toward server")
	}
}
