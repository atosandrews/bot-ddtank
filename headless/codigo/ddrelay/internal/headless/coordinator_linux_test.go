//go:build linux

package headless

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCoordinatorProcessLock(t *testing.T) {
	if directory := os.Getenv("DDRELAY_COORDINATOR_HELPER"); directory != "" {
		_, err := NewCoordinator(directory).Do(context.Background(), "s1", func(context.Context) error {
			if err := os.WriteFile(filepath.Join(directory, "held"), nil, 0600); err != nil {
				return err
			}
			select {}
		})
		t.Fatal(err)
	}
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestCoordinatorProcessLock$")
	child.Env = append(os.Environ(), "DDRELAY_COORDINATOR_HELPER="+directory)
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = child.Process.Kill(); _ = child.Wait() })
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(directory, "held")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child did not acquire process lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	_, err := NewCoordinator(directory).Do(ctx, "s1", func(context.Context) error { t.Error("process lock ignored"); return nil })
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected cancellable process lock: %v", err)
	}
	_ = child.Process.Kill()
	_ = child.Wait()
	ctx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if _, err := NewCoordinator(directory).Do(ctx, "s1", func(context.Context) error { return nil }); err != nil {
		t.Fatalf("dead process kept lock: %v", err)
	}
}
