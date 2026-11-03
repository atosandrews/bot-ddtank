package session

import "context"

type observationRequest struct {
	context context.Context
	reply   chan observationReply
}
