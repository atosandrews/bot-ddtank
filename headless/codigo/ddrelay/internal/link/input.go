package link

import (
	"context"

	pb "ddbot/api/gen/go/ddbot/v1"
)

type input struct {
	message *pb.RelayMessage
	context context.Context
	request *request
	size    int
}
type result struct {
	value *pb.ScriptResult
	err   error
}
