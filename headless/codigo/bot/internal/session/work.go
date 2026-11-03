package session

import (
	"context"

	pb "ddbot/api/gen/go/ddbot/v1"
)

type work struct {
	context    context.Context
	generation uint64
	message    *pb.RelayMessage
	attach     bool
	tick       bool
	resume     *pb.ResumeSession
	reply      chan reply
}
