package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ddbot/relay/internal/headless"
	"go.uber.org/fx"
)

type headlessRoundTrip func(*http.Request) (*http.Response, error)

func (f headlessRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type headlessShutdown struct{}

func (headlessShutdown) Shutdown(...fx.ShutdownOption) error { return nil }

// A blocked remote login must not fail process startup and trigger the container restart loop.
func TestHeadlessStartsWhileLoginIsBlocked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(path, []byte(`{"cookie":"offline-test-cookie-value"}`), 0600); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: headlessRoundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 403, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("forbidden"))}, nil
	})}
	auth, err := headless.NewAuthenticator(client)
	if err != nil {
		t.Fatal(err)
	}
	config := headless.Config{SessionFile: path, Characters: []headless.Character{{ID: 1, WebsiteID: 2, Name: "offline", Upstream: "127.0.0.1:1"}}}
	runtime := newHeadlessRuntime(config, auth,
		headless.NewPeer(auth, &net.Dialer{}, slog.New(slog.NewTextHandler(io.Discard, nil))), slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := runtime.start(ctx, headlessShutdown{}); err != nil {
		t.Fatalf("remote login failure must be handled by workers, not process startup: %v", err)
	}
	if err := runtime.stop(); err != nil {
		t.Fatal(err)
	}
}

type offlineAuthenticator func(context.Context, headless.AccountSession, headless.Character) (headless.Ticket, error)

func (f offlineAuthenticator) Authenticate(ctx context.Context, session headless.AccountSession, c headless.Character) (headless.Ticket, error) {
	return f(ctx, session, c)
}

type offlineGamePeer func(context.Context, headless.Ticket, func()) error

func (f offlineGamePeer) RunWithReady(ctx context.Context, ticket headless.Ticket, ready func()) error {
	return f(ctx, ticket, ready)
}

func offlineRuntime(t *testing.T) *headlessRuntime {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(path, []byte(`{"cookie":"first-offline-cookie"}`), 0600); err != nil {
		t.Fatal(err)
	}
	auth, err := headless.NewAuthenticator(&http.Client{})
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := newHeadlessRuntime(headless.Config{SessionFile: path, Characters: []headless.Character{{ID: 1, WebsiteID: 2, Name: "offline"}}}, auth, headless.NewPeer(auth, &net.Dialer{}, logger), logger)
	r.cancel = func() {}
	return r
}

func awaitOffline(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("offline worker did not progress")
	}
}

// Reloading is required because the external refresher replaces the session file.
func TestHeadlessReconnectReadsRenewedSession(t *testing.T) {
	r := offlineRuntime(t)
	cookies := []string{}
	r.authenticator = offlineAuthenticator(func(_ context.Context, s headless.AccountSession, c headless.Character) (headless.Ticket, error) {
		cookies = append(cookies, s.Cookie)
		if len(cookies) == 2 {
			return headless.Ticket{}, &headless.Error{Kind: "blocked"}
		}
		return headless.Ticket{Character: c}, nil
	})
	r.peer = offlineGamePeer(func(_ context.Context, _ headless.Ticket, ready func()) error {
		ready()
		if err := os.WriteFile(r.config.SessionFile, []byte(`{"cookie":"renewed-offline-cookie"}`), 0600); err != nil {
			t.Fatal(err)
		}
		return errors.New("offline disconnect")
	})
	r.pause = func(context.Context, time.Duration) bool { return len(cookies) < 2 }
	r.keep(context.Background(), r.config.Characters[0])
	if len(cookies) != 2 || cookies[0] != "first-offline-cookie" || cookies[1] != "renewed-offline-cookie" {
		t.Fatalf("sessions used: %v", cookies)
	}
	r.statusMu.Lock()
	defer r.statusMu.Unlock()
	if r.states[1].Phase != "blocked" {
		t.Fatalf("state: %+v", r.states[1])
	}
}

// A blocked character must not cancel an established connection for another character.
func TestHeadlessBlockPreservesHealthyConnection(t *testing.T) {
	r := offlineRuntime(t)
	r.config.Characters = append(r.config.Characters, headless.Character{ID: 3, WebsiteID: 4, Name: "blocked"})
	r.authenticator = offlineAuthenticator(func(_ context.Context, _ headless.AccountSession, c headless.Character) (headless.Ticket, error) {
		if c.ID == 3 {
			return headless.Ticket{}, &headless.Error{Kind: "blocked"}
		}
		return headless.Ticket{Character: c}, nil
	})
	online := make(chan struct{})
	closed := make(chan struct{})
	r.peer = offlineGamePeer(func(ctx context.Context, _ headless.Ticket, ready func()) error {
		ready()
		close(online)
		<-ctx.Done()
		close(closed)
		return nil
	})
	r.pause = func(context.Context, time.Duration) bool { return false }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go r.keep(ctx, r.config.Characters[0])
	awaitOffline(t, online)
	r.keep(ctx, r.config.Characters[1])
	select {
	case <-closed:
		t.Fatal("healthy peer closed on another character's HTTP block")
	default:
	}
	r.statusMu.Lock()
	state := r.states[1]
	r.statusMu.Unlock()
	if state.Phase != "online" {
		t.Fatalf("healthy state: %+v", state)
	}
	cancel()
	awaitOffline(t, closed)
}

func TestHeadlessShutdownCancelsAuthentication(t *testing.T) {
	r := offlineRuntime(t)
	entered := make(chan struct{})
	r.authenticator = offlineAuthenticator(func(ctx context.Context, _ headless.AccountSession, _ headless.Character) (headless.Ticket, error) {
		close(entered)
		<-ctx.Done()
		return headless.Ticket{}, ctx.Err()
	})
	if err := r.start(context.Background(), headlessShutdown{}); err != nil {
		t.Fatal(err)
	}
	awaitOffline(t, entered)
	stopped := make(chan struct{})
	go func() { _ = r.stop(); close(stopped) }()
	awaitOffline(t, stopped)
}

func TestHeadlessWaitsForChangedCookieAfterSessionExpiry(t *testing.T) {
	r := offlineRuntime(t)
	calls := 0
	polls := 0
	r.authenticator = offlineAuthenticator(func(_ context.Context, s headless.AccountSession, c headless.Character) (headless.Ticket, error) {
		calls++
		if calls == 1 {
			return headless.Ticket{}, &headless.Error{Kind: "session"}
		}
		if s.Cookie != "replacement-cookie-value" {
			t.Fatalf("retried before session changed: %q", s.Cookie)
		}
		return headless.Ticket{}, &headless.Error{Kind: "blocked"}
	})
	r.pause = func(context.Context, time.Duration) bool {
		if calls > 1 {
			return false
		}
		polls++
		if polls == 3 {
			if err := os.WriteFile(r.config.SessionFile, []byte(`{"cookie":"replacement-cookie-value"}`), 0600); err != nil {
				t.Fatal(err)
			}
		}
		return true
	}
	r.keep(context.Background(), r.config.Characters[0])
	if calls != 2 || polls != 3 {
		t.Fatalf("HTTP attempts=%d session polls=%d", calls, polls)
	}
}

func TestHeadlessConfigurationFailureIsOperational(t *testing.T) {
	r := offlineRuntime(t)
	r.authenticator = offlineAuthenticator(func(context.Context, headless.AccountSession, headless.Character) (headless.Ticket, error) {
		return headless.Ticket{}, &headless.Error{Kind: "config", Reason: "invalid transport"}
	})
	r.keep(context.Background(), r.config.Characters[0])
	if r.err == nil {
		t.Fatal("configuration error was hidden by retry")
	}
	if r.states[1].Phase != "failed" {
		t.Fatalf("state %+v", r.states[1])
	}
}
