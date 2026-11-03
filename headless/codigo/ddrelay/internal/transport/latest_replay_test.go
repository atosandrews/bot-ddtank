package transport

import (
	"os"
	"testing"
)

func TestPrivateLatestTurn(t *testing.T) {
	path := os.Getenv("DDTANK_LATEST_CAPTURE")
	if path == "" {
		t.Skip("private capture not configured")
	}
	r := replaySoloScript(t, path, "5f0abb9f5bf0a5416f98affcedd9e734ba02fb4ab47c05b5e583789c0e8584ae", "testdata/oct25_audit.lua")

	if r.frames != 523 || r.props != 2 || r.shots != 1 {
		t.Fatalf("frames=%d props=%d shots=%d", r.frames, r.props, r.shots)
	}
	turns, ok := r.audit["turns"].([]any)
	if !ok || len(turns) != 2 {
		t.Fatal("expected two own turns")
	}
	for _, row := range turns {
		turn, ok := row.(map[string]any)
		if !ok || turn["wind"] != true || turn["position"] != true || turn["moving"] != false || turn["weapon"] != true || turn["team"] != true {
			t.Fatal("own turn prerequisites unavailable")
		}
	}
	if r.audit["phase"] == "paused" {
		t.Fatal("optional buff blocked progress")
	}
	t.Log("523 unchanged frames; both player turns have numeric wind; two offline free-prop requests; one shot; second turn ends before timeout")
}
