package session

import (
	"context"
	"time"

	pb "ddbot/api/gen/go/ddbot/v1"
)

const maxObservedActors = 1024
const observationTimeout = 2 * time.Second

func (r *Registry) Observation(ctx context.Context, tenant, id string) (*pb.GetSessionObservationResponse, error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()

		return nil, &Error{Code: "service_closed"}
	}

	for key, entry := range r.entries {
		if key.tenant != tenant || entry.id != id || entry.session == nil {
			continue
		}

		owner := entry.session
		r.mu.Unlock()

		return owner.observe(ctx)
	}

	r.mu.Unlock()

	return nil, &Error{Code: "session_not_found"}
}

func (s *Session) observe(ctx context.Context) (*pb.GetSessionObservationResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, observationTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if !s.observationBusy.CompareAndSwap(false, true) {
		return nil, &Error{Code: "observation_busy"}
	}
	defer s.observationBusy.Store(false)

	request := observationRequest{context: ctx, reply: make(chan observationReply, 1)}
	select {
	case s.observations <- request:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.done:
		return nil, &Error{Code: "session_closed"}
	}

	select {
	case result := <-request.reply:
		return result.snapshot, result.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.done:
		return nil, &Error{Code: "session_closed"}
	}
}

func (s *Session) respondObservation(request observationRequest) {
	if err := request.context.Err(); err != nil {
		request.reply <- observationReply{err: err}

		return
	}

	snapshot, err := s.observation()
	request.reply <- observationReply{snapshot: snapshot, err: err}
}
