package transport

import (
	"context"

	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/relay/internal/link"
)

type Automation interface {
	Run(context.Context)
	Finish(context.Context) error
	Done() <-chan struct{}
	State() link.State
	Observe(*pb.PacketEvent)
	Receipt(*pb.CommandReceipt)
	Commands() <-chan *pb.CommandBatch
	Request(context.Context, string, string, string) (*pb.ScriptResult, error)
	Invalidate(string)
}
