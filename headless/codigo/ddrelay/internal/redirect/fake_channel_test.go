package redirect

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"testing"
)

type fakeChannel struct {
	input  chan []byte
	output chan []byte
}

func (c *fakeChannel) ReadMsgUnix(data, rights []byte) (int, int, int, *net.UnixAddr, error) {
	message, ok := <-c.input
	if !ok {
		return 0, 0, 0, nil, io.EOF
	}

	return copy(data, message), 0, 0, nil, nil
}

func (c *fakeChannel) WriteMsgUnix(data, rights []byte, address *net.UnixAddr) (int, int, error) {
	c.output <- append([]byte(nil), data...)

	return len(data), len(rights), nil
}

func TestHelperEOFRemovesOnlyOwnedRedirect(t *testing.T) {
	channel := &fakeChannel{input: make(chan []byte, 1), output: make(chan []byte, 1)}
	runner := &fakeRunner{}
	done := make(chan error, 1)
	go func() { done <- serveHelper(context.Background(), channel, 1000, runner) }()
	channel.input <- []byte(`{"operation":"configure","config":{"uid":1000,"port":11401,"ports":[1401],"mark":56698}}`)
	var response Message
	if err := json.Unmarshal(<-channel.output, &response); err != nil {
		t.Fatal(err)
	}

	if response.Failure != "" || response.Table == "" {
		t.Fatalf("configuration = %#v", response)
	}

	close(channel.input)
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	if runner.present || len(runner.inputs) != 2 || runner.inputs[1] != "delete table ip "+response.Table+"\n" {
		t.Fatalf("cleanup = %#v", runner.inputs)
	}
}

func TestHelperRejectsOtherUsersWithoutInstallingRules(t *testing.T) {
	runner := &fakeRunner{}
	var manager *Manager
	_, err := helperOperation(context.Background(), Message{Operation: "configure", Config: &Config{
		UID:   1001,
		Port:  11401,
		Ports: []uint16{1401},
		Mark:  1,
	}}, nil, 1000, &manager, runner)
	if err == nil || len(runner.inputs) != 0 || manager != nil {
		t.Fatalf("manager = %#v, transactions = %#v, error = %v", manager, runner.inputs, err)
	}
}
