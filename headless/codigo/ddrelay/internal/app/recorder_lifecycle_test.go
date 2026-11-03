package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRecordingStartsWithLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.jsonl")
	lifecycle := &fakeLifecycle{}
	recorder := newRecorder(ProxyConfig{RecordPath: path}, lifecycle)
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("recording opened during construction: %v", err)
	}

	if len(lifecycle.hooks) != 1 {
		t.Fatalf("hooks = %d, want 1", len(lifecycle.hooks))
	}

	hook := lifecycle.hooks[0]
	if err := hook.OnStart(context.Background()); err != nil {
		t.Fatal(err)
	}

	if recorder.Destination() != path {
		t.Fatalf("destination = %q, want %q", recorder.Destination(), path)
	}

	if err := hook.OnStop(context.Background()); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Size() != 0 || info.Mode().Perm() != 0600 {
		t.Fatalf("recording size=%d mode=%o", info.Size(), info.Mode().Perm())
	}
}
