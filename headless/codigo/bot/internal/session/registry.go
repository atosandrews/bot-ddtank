package session

import (
	"context"
	"crypto/rand"
	"sync"
	"time"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/bot/internal/script"
)

type Registry struct {
	mu      sync.Mutex
	entries map[identity]*entry
	options script.Options
	limits  Limits
	closed  bool
}

func NewRegistry(options script.Options, limits Limits) (*Registry, error) {
	if limits.Sessions < 1 || limits.PerTenant < 1 || limits.PerRelay < 1 || limits.Grace <= 0 || limits.Timeout <= 0 {
		return nil, &Error{Code: "invalid_limits"}
	}

	return &Registry{entries: make(map[identity]*entry), options: options, limits: limits}, nil
}

func (r *Registry) Open(ctx context.Context, tenant string, open *pb.OpenSession, resume *pb.ResumeSession) (*Session, []*pb.BotMessage, uint64, error) {
	if err := contract.ValidateOpen(open); err != nil {
		return nil, nil, 0, err
	}

	key := identity{tenant: tenant, relay: open.RelayInstanceId, game: open.GameSessionId}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()

		return nil, nil, 0, &Error{Code: "service_closed"}
	}

	current, exists := r.entries[key]
	if resume != nil {
		if exists && current.attached {
			r.mu.Unlock()

			return nil, nil, 0, &Error{Code: "session_attached", Reason: "previous stream has not detached"}
		}

		if !exists || current.attached || current.session == nil || current.id != resume.BotSessionId || current.generation != resume.Generation || time.Since(current.detached) > r.limits.Grace {
			r.mu.Unlock()

			return nil, nil, 0, &Error{Code: "resync_required", Reason: "no matching detached session"}
		}

		current.attached = true
		current.generation++
		generation := current.generation
		s := current.session
		r.mu.Unlock()
		messages, err := s.Bind(ctx, generation, resume)
		if err != nil {
			r.Release(tenant, open, generation)

			return nil, nil, 0, err
		}

		return s, messages, generation, nil
	}

	if exists {
		r.mu.Unlock()

		return nil, nil, 0, &Error{Code: "duplicate_session", Reason: "game connection already registered"}
	}

	tenantCount, relayCount := 0, 0

	for k := range r.entries {
		if k.tenant == tenant {
			tenantCount++
			if k.relay == open.RelayInstanceId {
				relayCount++
			}
		}
	}

	if len(r.entries) >= r.limits.Sessions || tenantCount >= r.limits.PerTenant || relayCount >= r.limits.PerRelay {
		r.mu.Unlock()

		return nil, nil, 0, &Error{Code: "session_limit"}
	}

	current = &entry{
		id:         rand.Text(),
		generation: 1,
		attached:   true,
	}
	r.entries[key] = current
	r.mu.Unlock()
	s, err := New(current.id, open.LocalStream, r.options, r.limits.Timeout)
	if err != nil {
		r.mu.Lock()
		delete(r.entries, key)
		r.mu.Unlock()

		return nil, nil, 0, err
	}

	r.mu.Lock()
	current.session = s
	closed := r.closed
	r.mu.Unlock()
	if closed {
		s.Close()

		return nil, nil, 0, &Error{Code: "service_closed"}
	}

	messages, err := s.Bind(ctx, 1, nil)
	if err != nil {
		r.Remove(tenant, open, 1)

		return nil, nil, 0, err
	}

	return s, messages, 1, nil
}

func (r *Registry) Release(tenant string, open *pb.OpenSession, generation uint64) {
	key := identity{tenant: tenant, relay: open.RelayInstanceId, game: open.GameSessionId}
	r.mu.Lock()
	e := r.entries[key]
	if e == nil || e.session == nil || e.generation != generation || !e.attached {
		r.mu.Unlock()

		return
	}

	owner := e.session
	r.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := owner.Detach(ctx, generation)

	removed := false
	r.mu.Lock()
	if r.entries[key] == e && e.generation == generation {
		e.attached = false
		e.detached = time.Now()
		if err != nil {
			delete(r.entries, key)
			removed = true
		}
	}

	r.mu.Unlock()

	if removed {
		owner.Close()
	}
}

func (r *Registry) Remove(tenant string, open *pb.OpenSession, generation uint64) {
	key := identity{tenant: tenant, relay: open.RelayInstanceId, game: open.GameSessionId}
	r.mu.Lock()
	e := r.entries[key]
	if e != nil && e.generation == generation {
		delete(r.entries, key)
	} else {
		e = nil
	}

	r.mu.Unlock()
	if e != nil && e.session != nil {
		e.session.Close()
	}
}

func (r *Registry) Expire(now time.Time) {
	var sessions []*Session
	r.mu.Lock()

	for key, e := range r.entries {
		if !e.attached && now.Sub(e.detached) >= r.limits.Grace {
			delete(r.entries, key)
			sessions = append(sessions, e.session)
		}
	}

	r.mu.Unlock()

	for _, s := range sessions {
		if s != nil {
			s.Close()
		}
	}
}

func (r *Registry) Close() {
	var sessions []*Session
	r.mu.Lock()
	r.closed = true

	for key, e := range r.entries {
		delete(r.entries, key)
		if e.session != nil {
			sessions = append(sessions, e.session)
		}
	}

	r.mu.Unlock()

	for _, s := range sessions {
		s.Close()
	}
}
