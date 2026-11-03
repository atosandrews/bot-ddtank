package control

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"
)

type closingListener struct {
	connection net.Conn
	closed     chan struct{}
}

func (l *closingListener) Accept() (net.Conn, error) {
	if l.connection != nil {
		connection := l.connection
		l.connection = nil

		return connection, nil
	}

	<-l.closed

	return nil, net.ErrClosed
}

func (l *closingListener) Close() error {
	close(l.closed)

	return nil
}

func (l *closingListener) Addr() net.Addr {
	return &net.UnixAddr{Name: "control-test", Net: "unix"}
}

func TestListenerClosureStopsIdleControlClients(t *testing.T) {
	client, peer := net.Pipe()
	defer client.Close()
	server := NewServer("", &fakeHandler{}, func(err error) { t.Error(err) })
	server.listener = &closingListener{connection: peer, closed: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	if err := json.NewEncoder(client).Encode(Request{ID: "inspect", Operation: "sessions"}); err != nil {
		t.Fatal(err)
	}

	var response Response
	if err := json.NewDecoder(client).Decode(&response); err != nil {
		t.Fatal(err)
	}

	if !response.OK || response.ID != "inspect" {
		t.Fatalf("response = %#v", response)
	}

	if err := server.Close(); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		cancel()
		<-done
		t.Fatal("listener closed but idle control client kept Serve alive")
	}
}
