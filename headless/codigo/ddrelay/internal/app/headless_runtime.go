package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"go.uber.org/fx"

	"ddbot/relay/internal/headless"
)

type headlessRuntime struct {
	config        headless.Config
	session       headless.AccountSession
	authenticator *headless.Authenticator
	peer          *headless.Peer
	logger        *slog.Logger
	cancel        context.CancelFunc
	wait          sync.WaitGroup
	done          chan struct{}
	mu            sync.Mutex
	err           error
}

func newHeadlessRuntime(config headless.Config, session headless.AccountSession, authenticator *headless.Authenticator, peer *headless.Peer, logger *slog.Logger) *headlessRuntime {
	return &headlessRuntime{config: config, session: session, authenticator: authenticator, peer: peer, logger: logger, done: make(chan struct{})}
}

func (r *headlessRuntime) start(startCtx context.Context, shutdown fx.Shutdowner) error {
	tickets := make([]headless.Ticket, 0, len(r.config.Characters))
	for _, character := range r.config.Characters {
		ticket, err := r.authenticator.Authenticate(startCtx, r.session, character)
		if err != nil {
			r.logger.Error("headless authentication failed", "character_id", character.ID, "error", err)

			return err
		}

		tickets = append(tickets, ticket)
		r.logger.Info("headless HTTP login confirmed", "character_id", character.ID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	for _, ticket := range tickets {
		r.wait.Add(1)
		go func() {
			defer r.wait.Done()
			if err := r.keep(ctx, ticket); err != nil {
				r.mu.Lock()
				r.err = errors.Join(r.err, err)
				r.mu.Unlock()
			}
		}()
	}

	go func() {
		defer close(r.done)
		r.wait.Wait()
		if err := shutdown.Shutdown(); err != nil {
			r.mu.Lock()
			r.err = errors.Join(r.err, err)
			r.mu.Unlock()
		}
	}()

	return nil
}

const (
	reconnectFirstDelay = 5 * time.Second
	reconnectMaxDelay   = time.Minute
	reconnectGiveUp     = 10 * time.Minute
	reconnectStable     = 5 * time.Minute
)

// keep holds one character online: when its game connection ends (server kick, network drop),
// it logs that character in again instead of leaving the party one member short. Delays grow
// from 5 s to 1 min; after 10 min without a working connection it gives up and returns the error.
func (r *headlessRuntime) keep(ctx context.Context, ticket headless.Ticket) error {
	delay := reconnectFirstDelay
	var failingSince time.Time
	for {
		started := time.Now()
		err := r.peer.Run(ctx, ticket)
		if ctx.Err() != nil {
			return nil
		}

		r.logger.Error("headless connection failed", "character_id", ticket.Character.ID, "error", err)
		if time.Since(started) >= reconnectStable {
			delay, failingSince = reconnectFirstDelay, time.Time{}
		}
		if failingSince.IsZero() {
			failingSince = time.Now()
		}
		if time.Since(failingSince) >= reconnectGiveUp {
			return err
		}

		for {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(delay):
			}
			delay = min(delay*2, reconnectMaxDelay)

			next, authErr := r.authenticator.Authenticate(ctx, r.session, ticket.Character)
			if authErr == nil {
				ticket = next
				r.logger.Info("headless reconnecting", "character_id", ticket.Character.ID)

				break
			}
			r.logger.Error("headless re-authentication failed", "character_id", ticket.Character.ID, "error", authErr)
			if time.Since(failingSince) >= reconnectGiveUp {
				return authErr
			}
		}
	}
}

func (r *headlessRuntime) stop() error {
	r.cancel()
	<-r.done
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.err
}
