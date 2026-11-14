package headless

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteStatusPublishesPrivateSnapshot(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "private")
	path := filepath.Join(directory, "login.json")
	next := time.Date(2026, 11, 13, 19, 5, 0, 0, time.UTC)
	if err := WriteStatus(path, Status{Characters: []CharacterStatus{{ID: 7, Phase: "blocked", Reason: "blocked", NextAttemptAt: &next}}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["version"] != float64(1) || decoded["pid"] != float64(os.Getpid()) {
		t.Fatalf("invalid identity: %s", data)
	}
	updated, err := time.Parse(time.RFC3339Nano, decoded["updated_at"].(string))
	if err != nil || time.Since(updated) > time.Minute {
		t.Fatalf("invalid heartbeat: %s", data)
	}
	character := decoded["characters"].([]any)[0].(map[string]any)
	if character["next_attempt_at"] != "2026-11-13T19:05:00Z" {
		t.Fatalf("invalid retry time: %s", data)
	}
	if _, ok := character["attempt_deadline"]; ok {
		t.Fatalf("unused deadline must be absent: %s", data)
	}
	for _, check := range []struct {
		path string
		mode os.FileMode
	}{{directory, 0700}, {path, 0600}} {
		info, err := os.Stat(check.path)
		if err != nil || info.Mode().Perm() != check.mode {
			t.Fatalf("permissions for %s: %v %v", check.path, info, err)
		}
	}
}

func TestWriteStatusReplacesSnapshotAndLeavesNoTemporaryFiles(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "login.json")
	for _, phase := range []string{"online", "stopped"} {
		if err := WriteStatus(path, Status{Characters: []CharacterStatus{{ID: 7, Phase: phase}}}); err != nil {
			t.Fatal(err)
		}
	}
	var snapshot Status
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Characters[0].Phase != "stopped" {
		t.Fatalf("old state survived replacement: %s", data)
	}
	entries, _ := os.ReadDir(directory)
	if len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v", entries)
	}
}

func TestWriteStatusRejectsPublicParentAndSymlink(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0755); err != nil {
		t.Fatal(err)
	}
	if err := WriteStatus(filepath.Join(directory, "status.json"), Status{}); err == nil {
		t.Fatal("public status parent accepted")
	}
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(directory, "target")
	if err := os.WriteFile(target, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "status.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := WriteStatus(link, Status{}); err == nil {
		t.Fatal("symlink output accepted")
	}
	data, _ := os.ReadFile(target)
	if string(data) != "preserve" {
		t.Fatal("symlink target modified")
	}
}
