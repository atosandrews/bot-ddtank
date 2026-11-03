package control

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/relay/internal/transport"
)

type fakeHandler struct {
	calls    []string
	injected []Request
}

func (f *fakeHandler) Inject(ctx context.Context, request Request) (int, error) {
	if request.Session != 3 {
		return 0, &transport.ControlError{Code: "session_unavailable", Reason: "unknown session"}
	}

	f.injected = append(f.injected, request)

	return len(request.Body) / 2, nil
}

func (f *fakeHandler) Sessions(context.Context) ([]transport.Info, error) {
	return []transport.Info{{
		Session:     3,
		Client:      "local",
		Upstream:    "game",
		CipherReady: true,
		GameReady:   true,
	}}, nil
}

func (f *fakeHandler) Request(ctx context.Context, request Request) (*pb.ScriptResult, error) {
	if request.Session != 3 {
		return nil, &transport.ControlError{Code: "session_unavailable", Reason: "unknown session"}
	}

	f.calls = append(f.calls, request.Code)
	value, err := contract.EncodeValue(map[string]any{"phase": "active"})
	if err != nil {
		return nil, err
	}

	return &pb.ScriptResult{RequestId: request.ID, Value: value}, nil
}

func TestControlRequestsSelectSessionAndKeepConnectionAfterError(t *testing.T) {
	handler := &fakeHandler{}
	server := NewServer("", handler, func(err error) { t.Error(err) })
	client, peer := net.Pipe()
	defer client.Close()
	done := make(chan error, 1)
	go func() { done <- server.Handle(context.Background(), peer) }()
	encoder, decoder := json.NewEncoder(client), json.NewDecoder(client)

	for _, tt := range []struct {
		request string
		ok      bool
		code    string
	}{
		{`{"id":"1","op":"eval","session":9,"code":"return room.state()"}`, false, "session_unavailable"},
		{`{"id":"2","op":"eval","session":3,"code":"return fight.state()"}`, true, ""},
		{`{"id":"3","op":"bogus"}`, false, "invalid_request"},
		{`{"id":"4","op":"sessions","unknown":1}`, false, "invalid_request"},
		{`{"id":"5","op":"inject","session":3,"packet_code":91,"body":"0600"}`, true, ""},
		{`{"id":"6","op":"inject","session":3,"packet_code":91,"body":"0"}`, false, "invalid_request"},
		{`{"id":"7","op":"inject","session":3,"packet_code":91,"body":"0600","code":"return 1"}`, false, "invalid_request"},
		{`{"id":"8","op":"inject","session":9,"packet_code":91,"body":"0600"}`, false, "session_unavailable"},
	} {
		if err := encoder.Encode(json.RawMessage(tt.request)); err != nil {
			t.Fatal(err)
		}

		var response Response
		if err := decoder.Decode(&response); err != nil {
			t.Fatal(err)
		}

		if response.OK != tt.ok {
			t.Fatalf("response = %#v", response)
		}

		if !tt.ok && response.Error.Code != tt.code {
			t.Fatalf("failure = %#v", response.Error)
		}
	}

	if err := client.Close(); err != nil {
		t.Fatal(err)
	}

	if err := <-done; err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(handler.calls, []string{"return fight.state()"}) {
		t.Fatalf("calls = %#v", handler.calls)
	}

	if len(handler.injected) != 1 || handler.injected[0].Body != "0600" || handler.injected[0].PacketCode != 91 {
		t.Fatalf("injected = %#v", handler.injected)
	}
}

func TestUnixSocketPermissionsCollisionAndCleanup(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "private")
	path := filepath.Join(directory, "control.sock")
	server := NewServer(path, &fakeHandler{}, func(err error) { t.Error(err) })
	if err := server.Open(); err != nil {
		if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
			t.Skipf("Unix listener unavailable in this environment: %v", err)
		}

		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("socket mode = %v, err = %v", info, err)
	}

	other := NewServer(path, &fakeHandler{}, func(err error) { t.Error(err) })
	if err := other.Open(); err == nil {
		t.Fatal("replaced active socket")
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	client, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}

	if err := json.NewEncoder(client).Encode(Request{ID: "sessions", Operation: "sessions"}); err != nil {
		t.Fatal(err)
	}

	var response Response
	if err := json.NewDecoder(client).Decode(&response); err != nil {
		t.Fatal(err)
	}

	if !response.OK || response.ID != "sessions" {
		t.Fatalf("response = %#v", response)
	}

	if err := client.Close(); err != nil {
		t.Fatal(err)
	}

	cancel()
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}

	if err := <-done; err != nil {
		t.Fatal(err)
	}

	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("socket remains: %v", err)
	}
}
