package transport

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	"ddbot/relay/internal/capture"
	"ddbot/relay/internal/framing"
	"ddbot/relay/internal/link"
	"ddbot/relay/internal/packets"
)

// Readiness metadata must remain available when Lua projection requests fail or are exhausted.
func TestControlSnapshotIdentifiesAcceptedCharacterWithoutScript(t *testing.T) {
	client, proxyClient := net.Pipe()
	server, proxyServer := net.Pipe()
	defer client.Close()
	defer proxyClient.Close()
	defer server.Close()
	defer proxyServer.Close()
	connection := NewConnection(1, proxyClient, proxyServer, &memoryRecorder{}, nil, packets.NewDecoder(),
		func(func(context.Context) (link.Checkpoint, error)) Automation { return &fakeAutomation{} })
	ctx := context.Background()
	key := [8]byte{11, 23, 31, 43, 59, 71, 83, 97}
	accepted := encrypted(t, &key, framing.Frame{Header: framing.Header{Code: 1, ClientID: 123}, Body: loginBody()})
	heartbeat := encrypted(t, &key, framing.Frame{Header: framing.Header{Code: 4}, Body: []byte{0, 0, 0, 2}})
	if err := connection.observe(ctx, capture.Record{Direction: capture.ServerToClient, Data: append(accepted, heartbeat...)}); err != nil {
		t.Fatal(err)
	}
	inspect := func() map[string]any {
		t.Helper()
		reply := make(chan controlReply, 1)
		if err := connection.handleControl(controlCall{context: ctx, operation: inspectOperation, response: reply}); err != nil {
			t.Fatal(err)
		}
		wire, err := json.Marshal((<-reply).info)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		if err := json.Unmarshal(wire, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	info := inspect()
	if info["game_ready"] != true || info["character_id"] != float64(123) {
		t.Fatalf("accepted character identity missing from control snapshot: %#v", info)
	}
	rejected := encrypted(t, &key, framing.Frame{Header: framing.Header{Code: 1, ClientID: 456}, Body: []byte{2}})
	if err := connection.observe(ctx, capture.Record{Direction: capture.ServerToClient, Data: rejected}); err != nil {
		t.Fatal(err)
	}
	info = inspect()
	if info["game_ready"] != false || info["character_id"] != float64(0) {
		t.Fatalf("rejected login retained a ready character: %#v", info)
	}
}
