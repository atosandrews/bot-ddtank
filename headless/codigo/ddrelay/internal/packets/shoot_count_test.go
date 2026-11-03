package packets

import (
	"reflect"
	"testing"

	"ddbot/relay/internal/capture"
	"ddbot/relay/internal/framing"
)

func TestShootCount(t *testing.T) {
	result := NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: 91, Parameter1: 7}, Body: []byte{46, 2}})
	want := map[string]any{"subcommand": uint8(46), "event": "CHANGE_SHOOT_COUNT", "living_id": int32(7), "shoot_count": int8(2)}
	if result.Status != "decoded" || result.Consumed != 2 || !reflect.DeepEqual(result.Fields, want) {
		t.Fatalf("result=%+v", result)
	}
}
