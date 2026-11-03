package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"ddbot/relay/internal/control"
)

func runControl(config ControlConfig, output io.Writer) (result error) {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	connection, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", config.Socket)
	if err != nil {
		return &control.Error{Operation: "connect", Cause: err}
	}

	var closeOnce sync.Once
	var closeErr error
	closeConnection := func() { closeOnce.Do(func() { closeErr = connection.Close() }) }
	defer func() { closeConnection(); result = errors.Join(result, closeErr) }()
	stop := context.AfterFunc(ctx, closeConnection)
	defer stop()

	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return &control.Error{Operation: "request id", Cause: err}
	}

	request := control.Request{
		ID:         hex.EncodeToString(id[:]),
		Operation:  config.Operation,
		Session:    config.Session,
		Code:       config.Code,
		Body:       config.Body,
		PacketCode: config.PacketCode,
		ClientID:   config.ClientID,
		Parameter1: config.Parameter1,
		Parameter2: config.Parameter2,
	}
	if config.RequestID != "" {
		request.ID = config.RequestID
	}

	if config.Sessions {
		request.Operation = "sessions"
	}

	if err := connection.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return &control.Error{Operation: "deadline", Cause: err}
	}

	if err := json.NewEncoder(connection).Encode(request); err != nil {
		return &control.Error{Operation: "write request", Cause: err}
	}

	var response control.Response
	if err := json.NewDecoder(io.LimitReader(connection, 1048576)).Decode(&response); err != nil {
		return &control.Error{Operation: "read response", Cause: err}
	}

	if err := json.NewEncoder(output).Encode(response); err != nil {
		return &control.Error{Operation: "output", Cause: err}
	}

	if response.ID != request.ID {
		return &control.Error{Operation: "response", Reason: "request id mismatch"}
	}

	if !response.OK {
		return &control.Error{Operation: "request", Reason: "proxy rejected the request; see JSON response"}
	}

	return nil
}
