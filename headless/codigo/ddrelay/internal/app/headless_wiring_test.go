package app

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"ddbot/relay/internal/headless"
	"go.uber.org/fx"
)

// Check the real CLI dependency graph without starting any remote workers.
func TestHeadlessDependencyGraph(t *testing.T) {
	err := fx.ValidateApp(fx.NopLogger,
		fx.Supply(headless.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil))),
		fx.Provide(newHeadlessDialer, newHeadlessAuthenticator, headless.NewPeer, newHeadlessRuntime),
		fx.Invoke(registerHeadlessLifecycle))
	if err != nil {
		t.Fatal(err)
	}
}

func TestHeadlessCheckReportsScheduledCooldownWithoutNetwork(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "auth")
	gate := headless.NewCoordinator(directory)
	next, err := gate.Do(context.Background(), "s1.ddclassico.com", func(context.Context) error { return &headless.Error{Kind: "forbidden", StatusCode: 403} })
	if err == nil || next.IsZero() {
		t.Fatalf("offline gate was not closed: %v", err)
	}
	var logs bytes.Buffer
	config := headless.Config{SessionFile: filepath.Join(directory, "missing-session.json"), Auth: headless.AuthConfig{Transport: "standard", CoordinationDir: directory}, Characters: []headless.Character{{ID: 1, WebsiteID: 2, Name: "offline"}}}
	if err := checkHeadlessAuth(config, slog.New(slog.NewJSONHandler(&logs, nil))); err == nil {
		t.Fatal("closed host gate was bypassed")
	}
	if !strings.Contains(logs.String(), `"next_attempt_at"`) {
		t.Fatalf("diagnostic omitted retry schedule: %s", logs.String())
	}
}

func TestHeadlessRejectsUnknownTransportBeforeReadingSession(t *testing.T) {
	_, err := newHeadlessAuthenticator(headless.Config{Auth: headless.AuthConfig{Transport: "unknown"}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil {
		t.Fatal("unknown transport accepted")
	}
}
