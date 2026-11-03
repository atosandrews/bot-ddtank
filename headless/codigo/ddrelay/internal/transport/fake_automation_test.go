package transport

import (
	"context"

	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/relay/internal/link"
)

type fakeAutomation struct {
	done     chan struct{}
	state    link.State
	receipts []*pb.CommandReceipt
	events   []*pb.PacketEvent
	invalid  string
}

func (f *fakeAutomation) Run(ctx context.Context) { <-ctx.Done(); close(f.done) }

func (f *fakeAutomation) Done() <-chan struct{} { return f.done }

func (f *fakeAutomation) State() link.State { return f.state }

func (f *fakeAutomation) Observe(event *pb.PacketEvent) { f.events = append(f.events, event) }

func (f *fakeAutomation) Receipt(receipt *pb.CommandReceipt) {
	f.receipts = append(f.receipts, receipt)
}

func (f *fakeAutomation) Commands() <-chan *pb.CommandBatch { return nil }

func (f *fakeAutomation) Request(context.Context, string, string, string) (*pb.ScriptResult, error) {
	return nil, &link.Error{Code: "detached"}
}

func (f *fakeAutomation) Invalidate(reason string) { f.invalid = reason }

func (f *fakeAutomation) Finish(context.Context) error { return nil }
