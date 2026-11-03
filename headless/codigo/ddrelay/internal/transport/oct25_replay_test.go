package transport

import (
	"os"
	"testing"
)

func TestPrivateOctober25TurnReadiness(t *testing.T) {
	path := os.Getenv("DDTANK_OCT25_CAPTURE")
	if path == "" {
		t.Skip("private October 25 prefix not configured")
	}

	result := replaySoloScript(t, path, "952f31a09078d9766d0ed20c6c1d14986e8b9bce38492dd754658e1edb806ff8", "testdata/oct25_audit.lua")
	if result.frames != 1480 || result.props != 4 || result.shots != 4 {
		t.Fatalf("frames=%d props=%d shots=%d", result.frames, result.props, result.shots)
	}

	turns, ok := result.audit["turns"].([]any)
	if !ok || len(turns) != 4 {
		t.Fatal("expected four player turns")
	}
	for _, row := range turns {
		turn, ok := row.(map[string]any)
		if !ok || turn["wind"] != true || turn["position"] != true || turn["moving"] != false || turn["weapon"] != true || turn["team"] != true {
			t.Fatal("player turn not ready")
		}
	}

	fires, ok := result.audit["fires"].([]any)
	if !ok || len(fires) != 4 {
		t.Fatal("expected four recorded FIRE values")
	}
	for i, expected := range []float64{6, 12, 13, 16} {
		fire, ok := fires[i].(map[string]any)
		if !ok || fire["display"] != expected || fire["reported"] != expected {
			t.Fatalf("turn %d wind disagrees with FIRE", i+1)
		}
	}
	if result.audit["phase"] == "paused" {
		t.Fatal("optional buff blocked progress")
	}

	t.Log("1480 frames; four numeric player winds match FIRE; four offline prop requests and four shots; no fabricated acceptance; capture unchanged")
}
