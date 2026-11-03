package transport

import (
	"context"

	"ddbot/relay/internal/framing"
)

const (
	inspectOperation controlOperation = iota + 1
	checkpointOperation
	injectOperation
)

type controlOperation uint8

type controlCall struct {
	context   context.Context
	operation controlOperation
	frame     framing.Frame
	response  chan controlReply
}
