package transport

import (
	"os"
	"reflect"
	"testing"
)

func TestPrivateSoloScriptContinuesWithoutBuffConfirmation(t *testing.T) {
	path := os.Getenv("DDTANK_SOLO_CAPTURE")
	if path == "" {
		t.Skip("private solo recording not configured")
	}

	result := replaySoloScript(t, path, "13aefb4c79cde66e604746b93e9514b0cc2074e92653a852b86699c4bdedd7c7", "testdata/solo_audit.lua")
	sequence, shots, props, report := result.frames, result.shots, result.props, result.audit

	if sequence != 496 || shots != 3 || props != 3 {
		t.Fatalf("frames=%d shots=%d props=%d audit=%v", sequence, shots, props, report)
	}

	turn := map[string]any{
		"wind":     true,
		"position": true,
		"moving":   false,
		"weapon":   true,
		"team":     true,
	}
	later := map[string]any{"wind": true, "position": true, "moving": false, "weapon": true, "team": true, "target_source": "server_destination"}
	if !reflect.DeepEqual(report["turns"], []any{turn, later, later}) {
		t.Fatalf("turn prerequisites=%v", report["turns"])
	}

	if report["phase"] != "fighting" || report["reason"] != "Waiting for the shot result and next turn" {
		t.Fatalf("unconfirmed prop outcome: %v", report)
	}

	t.Logf("%d frames, %d offline prop requests, %d shots without confirmation; no game socket opened; capture unchanged", sequence, props, shots)
}
