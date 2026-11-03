package link

import (
	"context"
	"crypto/sha256"
	"log/slog"
	"sync/atomic"
	"time"

	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
)

type Link struct {
	diagnostics   atomic.Pointer[Diagnostics]
	disconnects   atomic.Uint64
	scriptChanges atomic.Int64
	logger        *slog.Logger
	client        pb.BotClient
	key           string
	identity      *pb.OpenSession
	checkpoint    func(context.Context) (Checkpoint, error)
	input         chan input
	commands      chan *pb.CommandBatch
	done          chan struct{}
	state         atomic.Pointer[State]
	queued        atomic.Int64
	stopped       atomic.Bool
	closing       atomic.Bool
	source        string
}

func New(client pb.BotClient, key string, identity *pb.OpenSession, source string, logger *slog.Logger, checkpoint func(context.Context) (Checkpoint, error)) *Link {
	l := &Link{
		logger:     logger,
		client:     client,
		key:        key,
		identity:   identity,
		source:     source,
		checkpoint: checkpoint,
		input:      make(chan input, 128),
		commands:   make(chan *pb.CommandBatch, 8),
		done:       make(chan struct{}),
	}
	l.state.Store(&State{Reason: "connecting"})

	return l
}

func (l *Link) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer close(l.done)
	defer cancel()
	l.run(ctx)
}

func (l *Link) State() State {
	state := *l.state.Load()
	state.ChangingScript = l.scriptChanges.Load() > 0
	if d := l.diagnostics.Load(); d != nil {
		state.Diagnostics = *d
	}

	state.Diagnostics.InputQueued = len(l.input)
	state.Diagnostics.InputBytes = l.queued.Load()
	state.Diagnostics.Disconnects = l.disconnects.Load()
	if l.stopped.Load() {
		state.Attached = false
	}

	return state
}

func (l *Link) Commands() <-chan *pb.CommandBatch { return l.commands }

func (l *Link) Observe(event *pb.PacketEvent) {
	l.enqueue(input{message: &pb.RelayMessage{Payload: &pb.RelayMessage_Packet{Packet: event}}})
}

func (l *Link) Receipt(receipt *pb.CommandReceipt) {
	l.enqueue(input{message: &pb.RelayMessage{Payload: &pb.RelayMessage_Receipt{Receipt: receipt}}})
}

func (l *Link) Invalidate(reason string) {
	l.stopped.Store(true)
	old := l.State()
	old.Attached = false
	old.Reason = reason
	l.state.Store(&old)
}

func (l *Link) enqueue(item input) bool {
	if l.stopped.Load() {
		return false
	}

	item.size = proto.Size(item.message)
	if l.queued.Add(int64(item.size)) > contract.MaxHistoryBytes {
		l.queued.Add(-int64(item.size))
		l.Invalidate("link queue exceeds byte limit")

		return false
	}

	select {
	case l.input <- item:
		return true
	default:
		l.queued.Add(-int64(item.size))
		l.Invalidate("link queue is full")
		return false
	}
}

func (l *Link) Request(ctx context.Context, id, operation, source string) (*pb.ScriptResult, error) {
	if !l.State().Attached || l.stopped.Load() {
		return nil, &Error{Code: "session_detached", Reason: l.State().Reason}
	}

	message := &pb.RelayMessage{}

	switch operation {
	case "eval":
		message.Payload = &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: id, Source: source}}
	case "load":
		hash := sha256.Sum256([]byte(source))
		message.Payload = &pb.RelayMessage_Load{Load: &pb.LoadScript{RequestId: id, Source: source, Sha256: hash[:]}}
	case "stop":
		message.Payload = &pb.RelayMessage_Stop{Stop: &pb.StopScript{RequestId: id}}
	default:
		return nil, &Error{Code: "invalid_operation", Reason: operation}
	}

	if err := contract.ValidateRelay(message); err != nil {
		return nil, err
	}

	changing := operation == "load" || operation == "stop"
	request := newRequest(l, changing)
	defer request.release()

	if !l.enqueue(input{message: message, context: ctx, request: request}) {
		return nil, &Error{Code: "session_detached", Reason: l.State().Reason}
	}

	select {
	case r := <-request.reply:
		if changing && r.err != nil {
			l.Invalidate("script replacement outcome unknown")
		}
		return r.value, r.err
	case <-ctx.Done():
		if changing {
			l.Invalidate("script replacement outcome unknown")
		}
		return nil, &Error{Code: "outcome_unknown", Reason: "request deadline after submission; do not retry blindly"}
	case <-l.done:
		return nil, &Error{Code: "outcome_unknown", Reason: "game connection closed during request"}
	}
}

func (l *Link) connect(ctx context.Context, resume *pb.ResumeSession) (pb.Bot_GameSessionClient, *pb.SessionOpened, context.CancelFunc, error) {
	streamCtx, cancel := context.WithCancel(metadata.AppendToOutgoingContext(ctx, "x-api-key", l.key))
	timer := time.AfterFunc(10*time.Second, cancel)
	stream, err := l.client.GameSession(streamCtx)
	if err != nil {
		timer.Stop()
		cancel()

		return nil, nil, nil, err
	}

	opening := &pb.RelayMessage{Payload: &pb.RelayMessage_Open{Open: l.identity}}
	if resume != nil {
		opening.Payload = &pb.RelayMessage_Resume{Resume: resume}
	}

	if err := stream.Send(opening); err != nil {
		timer.Stop()
		cancel()

		return nil, nil, nil, err
	}

	answer, err := stream.Recv()
	timer.Stop()
	if err != nil {
		cancel()

		return nil, nil, nil, err
	}

	if err := contract.ValidateBot(answer); err != nil {
		cancel()

		return nil, nil, nil, &Error{Code: "invalid_open", Reason: "invalid bot acknowledgement", Cause: err}
	}

	opened := answer.GetOpened()
	if opened == nil || opened.BotSessionId == "" || opened.Generation == 0 || opened.ApiVersion != contract.APIVersion || opened.CodecRevision != contract.CodecRevision {
		cancel()

		return nil, nil, nil, &Error{Code: "invalid_open", Reason: "bot did not acknowledge opening"}
	}

	return stream, opened, cancel, nil
}

func (l *Link) Done() <-chan struct{} { return l.done }

func (l *Link) Finish(ctx context.Context) error {
	if l.closing.Swap(true) || !l.State().Attached {
		return nil
	}

	if !l.enqueue(input{message: &pb.RelayMessage{Payload: &pb.RelayMessage_Close{Close: &pb.Close{Reason: "game connection closed"}}}}) {
		return &Error{Code: "close_unconfirmed", Reason: "bot close notification could not be queued"}
	}

	select {
	case <-l.done:
		return nil
	case <-ctx.Done():
		return &Error{Code: "close_unconfirmed", Reason: "bot close notification was not confirmed", Cause: ctx.Err()}
	}
}
