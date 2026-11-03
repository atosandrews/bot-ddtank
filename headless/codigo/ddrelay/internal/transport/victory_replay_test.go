package transport

import (
	"os"
	"testing"
)

func TestPrivateVictoryScript(t *testing.T) {
	path := os.Getenv("DDTANK_VICTORY_CAPTURE")
	if path == "" {
		t.Skip("private victory prefix not configured")
	}

	r := replaySoloScript(t, path, "3368bd303d08c3693c9376fbd38f8c765667d03cf62275485c94ac88c3c45002", "testdata/victory_audit.lua")
	if r.frames != 2064 || r.props != 32 || r.shots != 29 || r.audit["phase"] != "done" || r.audit["wind_mismatches"] != float64(0) || r.audit["awards"] != float64(3) {
		t.Fatalf("frames=%d props=%d shots=%d audit=%v", r.frames, r.props, r.shots, r.audit)
	}
	shots, ok := r.audit["shots"].([]any)
	if !ok || len(shots) != 29 {
		t.Fatal("missing generated shot evidence")
	}
	for _, value := range shots {
		shot, ok := value.(map[string]any)
		if !ok || shot["fallback"] != false {
			t.Fatal("shot used unknown wind")
		}
	}
	if r.audit["turns"] != float64(32) || r.audit["fires"] != float64(16) || r.audit["reports"] != float64(32) || r.audit["cards_goal"] != float64(3) {
		t.Fatal("victory replay checkpoints differ")
	}
	t.Log("2064 unchanged frames; 29 generated shots with recognized wind; 16 FIRE comparisons match; three final awards")
}
