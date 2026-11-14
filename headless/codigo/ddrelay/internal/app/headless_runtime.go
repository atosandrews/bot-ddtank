package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"ddbot/relay/internal/headless"
	"go.uber.org/fx"
)

type headlessAuthenticator interface {
	Authenticate(context.Context, headless.AccountSession, headless.Character) (headless.Ticket, error)
}

type headlessGamePeer interface {
	RunWithReady(context.Context, headless.Ticket, func()) error
}

type headlessRuntime struct {
	config        headless.Config
	authenticator headlessAuthenticator
	peer          headlessGamePeer
	logger        *slog.Logger
	coordinator   *headless.Coordinator
	loadSession   func(string) (headless.AccountSession, error)
	pause         func(context.Context, time.Duration) bool
	cancel        context.CancelFunc
	wait          sync.WaitGroup
	done          chan struct{}
	mu            sync.Mutex
	err           error
	statusMu      sync.Mutex
	states        map[int32]headless.CharacterStatus
}

func newHeadlessRuntime(config headless.Config, authenticator *headless.Authenticator, peer *headless.Peer, logger *slog.Logger) *headlessRuntime {
	return &headlessRuntime{config: config, authenticator: authenticator, peer: peer, logger: logger,
		coordinator: headless.NewCoordinator(config.Auth.CoordinationDir), loadSession: headless.LoadAccountSession,
		pause: waitHeadless, done: make(chan struct{}), states: make(map[int32]headless.CharacterStatus)}
}

const (
	headlessAttemptTimeout = 60 * time.Second
	reconnectFirstDelay    = 5 * time.Second
	reconnectMaxDelay      = time.Minute
	reconnectStable        = 5 * time.Minute
)

// Startup establishes process health. Remote authentication happens outside Fx's startup deadline.
func (r *headlessRuntime) start(startCtx context.Context, shutdown fx.Shutdowner) error {
	if err := startCtx.Err(); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	for _, character := range r.config.Characters {
		deadline := time.Now().Add(headlessAttemptTimeout)
		r.states[character.ID] = headless.CharacterStatus{ID: character.ID, Phase: "queued", AttemptDeadline: &deadline}
	}
	if err := r.writeStatus(); err != nil {
		cancel()
		r.cancel = nil
		return err
	}
	r.wait.Add(1)
	go func() {
		defer r.wait.Done()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := r.writeStatus(); err != nil {
					r.fail(err)
					return
				}
			}
		}
	}()
	for _, character := range r.config.Characters {
		r.wait.Add(1)
		go func() { defer r.wait.Done(); r.keep(ctx, character) }()
	}
	go func() {
		r.wait.Wait()
		r.statusMu.Lock()
		for id := range r.states {
			r.states[id] = headless.CharacterStatus{ID: id, Phase: "stopped"}
		}
		err := r.writeStatusLocked()
		r.statusMu.Unlock()
		if err != nil {
			r.addError(err)
		}
		if err := shutdown.Shutdown(); err != nil {
			r.addError(err)
		}
		close(r.done)
	}()
	return nil
}

func (r *headlessRuntime) keep(ctx context.Context, character headless.Character) {
	delay := reconnectFirstDelay
	for ctx.Err() == nil {
		var session headless.AccountSession
		deadline := time.Now().Add(headlessAttemptTimeout)
		r.update(headless.CharacterStatus{ID: character.ID, Phase: "queued", AttemptDeadline: &deadline})
		attempt, cancel := context.WithDeadline(ctx, deadline)
		var ticket headless.Ticket
		next, err := r.coordinator.Do(attempt, "s1.ddclassico.com", func(authCtx context.Context) error {
			r.update(headless.CharacterStatus{ID: character.ID, Phase: "authenticating", AttemptDeadline: &deadline})
			var loadErr error
			session, loadErr = r.loadSession(r.config.SessionFile)
			if loadErr != nil {
				return &headless.Error{Operation: "account session", Kind: "session", Reason: "cannot load private session", Cause: loadErr}
			}
			var authErr error
			ticket, authErr = r.authenticator.Authenticate(authCtx, session, character)
			return authErr
		})
		cancel()
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			r.logger.Warn("headless authentication waiting", "character_id", character.ID, "error", err)
			var authError *headless.Error
			if errors.As(err, &authError) && authError.Kind == "config" {
				r.update(headless.CharacterStatus{ID: character.ID, Phase: "failed", Reason: "configuration"})
				r.fail(err)
				return
			}
			if errors.As(err, &authError) && (authError.Kind == "session" || authError.Kind == "rejected") {
				if !r.awaitSession(ctx, character.ID, session.Cookie, authError.Kind) {
					return
				}
				continue
			}
			if !next.IsZero() {
				r.update(headless.CharacterStatus{ID: character.ID, Phase: "blocked", Reason: "login_forbidden", NextAttemptAt: &next})
				if !r.pause(ctx, time.Until(next)) {
					return
				}
				continue
			}
			if !r.backoff(ctx, character.ID, "authentication", delay) {
				return
			}
			delay = min(delay*2, reconnectMaxDelay)
			continue
		}
		r.logger.Info("headless HTTP login confirmed", "character_id", character.ID)
		connectedAt := time.Now()
		gameDeadline := connectedAt.Add(headlessAttemptTimeout)
		r.update(headless.CharacterStatus{ID: character.ID, Phase: "connecting", AttemptDeadline: &gameDeadline})
		err = r.peer.RunWithReady(ctx, ticket, func() {
			r.update(headless.CharacterStatus{ID: character.ID, Phase: "online"})
		})
		if ctx.Err() != nil {
			return
		}
		r.logger.Warn("headless connection ended", "character_id", character.ID, "error", err)
		if time.Since(connectedAt) >= reconnectStable {
			delay = reconnectFirstDelay
		}
		if !r.backoff(ctx, character.ID, "connection", delay) {
			return
		}
		delay = min(delay*2, reconnectMaxDelay)
	}
}

func (r *headlessRuntime) awaitSession(ctx context.Context, id int32, previous, reason string) bool {
	r.update(headless.CharacterStatus{ID: id, Phase: "session_required", Reason: reason})
	for r.pause(ctx, 5*time.Second) {
		session, err := r.loadSession(r.config.SessionFile)
		if err == nil && session.Cookie != previous {
			return true
		}
	}
	return false
}

func (r *headlessRuntime) backoff(ctx context.Context, id int32, reason string, delay time.Duration) bool {
	next := time.Now().Add(delay)
	r.update(headless.CharacterStatus{ID: id, Phase: "backoff", Reason: reason, NextAttemptAt: &next})
	return r.pause(ctx, delay)
}

func waitHeadless(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(max(0, duration))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return ctx.Err() == nil
	}
}

func (r *headlessRuntime) update(state headless.CharacterStatus) {
	r.statusMu.Lock()
	r.states[state.ID] = state
	err := r.writeStatusLocked()
	r.statusMu.Unlock()
	if err != nil {
		r.fail(err)
	}
}

func (r *headlessRuntime) writeStatus() error {
	r.statusMu.Lock()
	defer r.statusMu.Unlock()
	return r.writeStatusLocked()
}

func (r *headlessRuntime) writeStatusLocked() error {
	if r.config.Auth.StatusFile == "" {
		return nil
	}
	states := make([]headless.CharacterStatus, 0, len(r.config.Characters))
	for _, character := range r.config.Characters {
		states = append(states, r.states[character.ID])
	}
	return headless.WriteStatus(r.config.Auth.StatusFile, headless.Status{Characters: states})
}

func (r *headlessRuntime) fail(err error) { r.addError(err); r.cancel() }
func (r *headlessRuntime) addError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = errors.Join(r.err, err)
}

func (r *headlessRuntime) stop() error {
	if r.cancel != nil {
		r.cancel()
		<-r.done
	}
	if closer, ok := r.authenticator.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}
