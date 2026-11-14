package headless

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCoordinatorBlocksRepeatedProbesAndEscalatesThenResets(t *testing.T) {
	c := NewCoordinator("")
	now := time.Date(2026, 11, 13, 18, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return now }
	c.jitter = func(delay time.Duration) time.Duration { return delay }
	blocked := &Error{Operation: "request", Kind: "blocked", Reason: "blocked"}
	next, err := c.Do(context.Background(), "s1.ddclassico.com", func(context.Context) error { return blocked })
	if err != blocked || !next.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("first wait: %v %v", next, err)
	}
	called := false
	_, err = c.Do(context.Background(), "s1.ddclassico.com", func(context.Context) error { called = true; return nil })
	var cooldown *CooldownError
	if !errors.As(err, &cooldown) || called || !cooldown.Until.Equal(next) {
		t.Fatalf("cooldown ignored: %v called=%v", err, called)
	}
	now = next
	next, _ = c.Do(context.Background(), "s1.ddclassico.com", func(context.Context) error { return blocked })
	if !next.Equal(now.Add(10 * time.Minute)) {
		t.Fatalf("second wait: %v", next)
	}
	now = next
	next, _ = c.Do(context.Background(), "s1.ddclassico.com", func(context.Context) error { return blocked })
	if !next.Equal(now.Add(15 * time.Minute)) {
		t.Fatalf("third wait: %v", next)
	}
	now = next
	if _, err := c.Do(context.Background(), "s1.ddclassico.com", func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	next, _ = c.Do(context.Background(), "s1.ddclassico.com", func(context.Context) error { return blocked })
	if !next.Equal(now.Add(5 * time.Minute)) {
		t.Fatalf("success did not reset gate: %v", next)
	}
}

func TestCoordinatorTransportFailureDoesNotResetBlockHistory(t *testing.T) {
	c := NewCoordinator("")
	now := time.Now()
	c.now = func() time.Time { return now }
	c.jitter = func(delay time.Duration) time.Duration { return delay }
	blocked := &Error{Kind: "forbidden", RetryAfter: 20 * time.Minute}
	next, _ := c.Do(context.Background(), "s1", func(context.Context) error { return blocked })
	if !next.Equal(now.Add(20 * time.Minute)) {
		t.Fatalf("Retry-After ignored: %v", next)
	}
	now = next
	_, err := c.Do(context.Background(), "s1", func(context.Context) error { return &Error{Kind: "network"} })
	if err == nil {
		t.Fatal("network error lost")
	}
	next, _ = c.Do(context.Background(), "s1", func(context.Context) error { return &Error{Kind: "challenge"} })
	if !next.Equal(now.Add(10 * time.Minute)) {
		t.Fatalf("network failure reset block history: %v", next)
	}
	if _, err := c.Do(context.Background(), "different-host", func(context.Context) error { return nil }); err != nil {
		t.Fatal("other host incorrectly blocked", err)
	}
}

func TestCoordinatorSerializesConcurrentAuthentication(t *testing.T) {
	c := NewCoordinator("")
	var inFlight, overlap atomic.Int32
	var wait sync.WaitGroup
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := c.Do(context.Background(), "s1", func(context.Context) error {
				if inFlight.Add(1) != 1 {
					overlap.Add(1)
				}
				time.Sleep(time.Millisecond)
				inFlight.Add(-1)
				return nil
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wait.Wait()
	if overlap.Load() != 0 {
		t.Fatal("authentication requests overlapped")
	}
}

func TestCoordinatorSharesCooldownAcrossInstancesAndRejectsInvalidState(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	c := NewCoordinator(directory)
	next, _ := c.Do(context.Background(), "s1", func(context.Context) error { return &Error{Kind: "blocked"} })
	called := false
	got, err := NewCoordinator(directory).Do(context.Background(), "s1", func(context.Context) error { called = true; return nil })
	var cooldown *CooldownError
	if called || !errors.As(err, &cooldown) || !got.Equal(next) {
		t.Fatalf("shared circuit ignored: %v %v", got, err)
	}
	files, _ := filepath.Glob(filepath.Join(directory, "*.json"))
	if len(files) != 1 {
		t.Fatalf("missing circuit snapshot: %v", files)
	}
	if err := os.WriteFile(files[0], []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = NewCoordinator(directory).Do(context.Background(), "s1", func(context.Context) error { called = true; return nil })
	if err == nil || called {
		t.Fatal("invalid shared state allowed an uncontrolled probe")
	}
}

func TestCoordinatorCancelsWaitingForAnotherAttempt(t *testing.T) {
	c := NewCoordinator("")
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		_, _ = c.Do(context.Background(), "s1", func(context.Context) error { close(started); <-release; return nil })
	}()
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Do(ctx, "s1", func(context.Context) error { t.Error("cancelled waiter executed request"); return nil })
	close(release)
	<-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("wait was not cancellable: %v", err)
	}
}
