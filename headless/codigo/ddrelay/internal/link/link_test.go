package link

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
)

func TestInputLimitsDoNotAffectAnotherLink(t *testing.T) {
	for _, tc := range []struct {
		name string
		fill func(*Link)
	}{
		{"count", func(l *Link) {
			for i := 0; i < 129; i++ {
				l.Observe(&pb.PacketEvent{Sequence: uint64(i + 1)})
			}
		}},
		{"bytes", func(l *Link) { l.queued.Store(contract.MaxHistoryBytes); l.Observe(&pb.PacketEvent{Sequence: 1}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stalled := New(nil, "", nil, "", slog.New(slog.NewJSONHandler(io.Discard, nil)), nil)
			healthy := New(nil, "", nil, "", slog.New(slog.NewJSONHandler(io.Discard, nil)), nil)
			tc.fill(stalled)
			healthy.Observe(&pb.PacketEvent{Sequence: 1})
			if !stalled.stopped.Load() || stalled.State().Attached || healthy.stopped.Load() || len(healthy.input) != 1 {
				t.Fatalf("stalled=%+v healthy=%+v queued=%d", stalled.State(), healthy.State(), len(healthy.input))
			}
		})
	}
}

func TestConnectedRejectsMissingHistoryAndStaleAcknowledgements(t *testing.T) {
	for _, tc := range []struct {
		name     string
		history  int
		sequence uint64
		ack      uint64
	}{
		{name: "history overflow", history: contract.MaxEvents, sequence: 1},
		{name: "sequence gap", sequence: 2},
		{name: "unobserved acknowledgement", ack: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := New(nil, "", nil, "", slog.New(slog.NewJSONHandler(io.Discard, nil)), nil)
			incoming := make(chan received, 1)
			if tc.ack != 0 {
				incoming <- received{message: &pb.BotMessage{Payload: &pb.BotMessage_Applied{Applied: &pb.EventsApplied{Sequence: tc.ack}}}}
			} else {
				l.Observe(&pb.PacketEvent{Sequence: tc.sequence})
			}

			history := make([]*pb.PacketEvent, tc.history)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			tick := time.NewTicker(time.Hour)
			defer tick.Stop()
			c := &connection{
				link:     l,
				opened:   &pb.SessionOpened{},
				incoming: incoming,
				pending:  map[string]*request{},
				history:  &eventHistory{events: history},
				tick:     tick,
			}
			err := c.run(ctx)
			if err == nil || !l.stopped.Load() || errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("error=%v state=%+v", err, l.State())
			}
		})
	}
}

func TestCachedResultCannotRevertScriptGeneration(t *testing.T) {
	l := New(nil, "", nil, "", slog.New(slog.NewJSONHandler(io.Discard, nil)), nil)
	l.state.Store(&State{Attached: true, Generation: 2, ScriptGeneration: 3})
	incoming := make(chan received, 2)
	incoming <- received{message: &pb.BotMessage{Payload: &pb.BotMessage_Result{Result: &pb.ScriptResult{RequestId: "old", ScriptGeneration: 1, Value: &pb.Value{Kind: &pb.Value_NullValue{NullValue: true}}}}}}
	incoming <- received{err: io.EOF}

	var history []*pb.PacketEvent

	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	c := &connection{
		link:     l,
		opened:   &pb.SessionOpened{},
		incoming: incoming,
		pending:  map[string]*request{},
		history:  &eventHistory{events: history},
		tick:     tick,
	}
	err := c.run(context.Background())
	if !errors.Is(err, io.EOF) || l.State().ScriptGeneration != 3 {
		t.Fatalf("state=%+v error=%v", l.State(), err)
	}
}

func TestSubmittedRequestDeadlineReportsUnknownOutcome(t *testing.T) {
	l := New(nil, "", nil, "", slog.New(slog.NewJSONHandler(io.Discard, nil)), nil)
	l.state.Store(&State{Attached: true})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := l.Request(ctx, "request", "eval", "return 1")
	var domain *Error
	if !errors.As(err, &domain) || domain.Code != "outcome_unknown" || len(l.input) != 1 {
		t.Fatalf("error=%v queued=%d", err, len(l.input))
	}
}

func TestScriptUploadPausesCommandWritesUntilResult(t *testing.T) {
	l := New(nil, "", nil, "", slog.New(slog.NewJSONHandler(io.Discard, nil)), nil)
	l.state.Store(&State{Attached: true, ScriptGeneration: 1})
	done := make(chan error, 1)
	go func() {
		_, err := l.Request(context.Background(), "load", "load", "return {api_version=1}")
		done <- err
	}()
	item := <-l.input
	if !l.State().ChangingScript {
		t.Fatal("upload did not pause command writes")
	}

	item.request.respond(result{value: &pb.ScriptResult{ScriptGeneration: 2}})
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	if l.State().ChangingScript {
		t.Fatal("confirmed upload remained paused")
	}
}
