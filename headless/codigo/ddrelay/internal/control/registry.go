package control

import (
	"context"
	"encoding/hex"
	"slices"
	"sync"

	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/relay/internal/framing"
	"ddbot/relay/internal/transport"
)

type Registry struct {
	mu          sync.RWMutex
	connections map[uint64]*transport.Connection
}

func NewRegistry() *Registry { return &Registry{connections: make(map[uint64]*transport.Connection)} }

func (r *Registry) Add(id uint64, connection *transport.Connection) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.connections[id] = connection
}

func (r *Registry) Remove(id uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.connections, id)
}

func (r *Registry) Sessions(ctx context.Context) ([]transport.Info, error) {
	r.mu.RLock()
	connections := make(map[uint64]*transport.Connection, len(r.connections))
	ids := make([]uint64, 0, len(r.connections))

	for id, connection := range r.connections {
		connections[id] = connection
		ids = append(ids, id)
	}

	r.mu.RUnlock()
	slices.Sort(ids)

	result := make([]transport.Info, 0, len(ids))

	for _, id := range ids {
		info, err := connections[id].Inspect(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			continue
		}

		result = append(result, info)
	}

	return result, nil
}

func (r *Registry) Request(ctx context.Context, request Request) (*pb.ScriptResult, error) {
	r.mu.RLock()
	connection, exists := r.connections[request.Session]
	r.mu.RUnlock()
	if !exists {
		return nil, &transport.ControlError{Code: "session_unavailable", Reason: "no active connection with this session id"}
	}

	return connection.Request(ctx, request.ID, request.Operation, request.Code)
}

func (r *Registry) Inject(ctx context.Context, request Request) (int, error) {
	body, err := hex.DecodeString(request.Body)
	if err != nil {
		return 0, &transport.ControlError{Code: "invalid_request", Reason: "body must be hexadecimal"}
	}

	r.mu.RLock()
	connection, exists := r.connections[request.Session]
	r.mu.RUnlock()
	if !exists {
		return 0, &transport.ControlError{Code: "session_unavailable", Reason: "no active connection with this session id"}
	}

	frame := framing.Frame{
		Header: framing.Header{
			Code:       int16(request.PacketCode),
			ClientID:   request.ClientID,
			Parameter1: request.Parameter1,
			Parameter2: request.Parameter2,
		},
		Body: body,
	}

	return connection.Inject(ctx, frame)
}
