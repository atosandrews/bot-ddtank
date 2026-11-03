package control

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/relay/internal/transport"
)

const MaxRequestBytes = 8 << 20

const MaxInjectBodyBytes = 60000

type Handler interface {
	Sessions(context.Context) ([]transport.Info, error)
	Request(context.Context, Request) (*pb.ScriptResult, error)
	Inject(context.Context, Request) (int, error)
}

type Request struct {
	ID         string `json:"id"`
	Operation  string `json:"op"`
	Session    uint64 `json:"session,omitempty"`
	Code       string `json:"code,omitempty"`
	Body       string `json:"body,omitempty"`
	PacketCode int32  `json:"packet_code,omitempty"`
	ClientID   int32  `json:"client_id,omitempty"`
	Parameter1 int32  `json:"parameter1,omitempty"`
	Parameter2 int32  `json:"parameter2,omitempty"`
}

type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Response struct {
	ID               string   `json:"id"`
	OK               bool     `json:"ok"`
	Value            any      `json:"value,omitempty"`
	ActionIDs        []uint64 `json:"action_ids,omitempty"`
	ScriptGeneration uint64   `json:"script_generation,omitempty"`
	Error            *Failure `json:"error,omitempty"`
}

func Decode(data []byte) (Request, error) {
	var request Request
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return request, &Error{Operation: "decode request", Cause: err}
	}

	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return request, &Error{Operation: "decode request", Reason: "expected one JSON object"}
	}

	if request.ID == "" || len(request.ID) > 128 {
		return request, &Error{Operation: "request", Reason: "id must contain 1 to 128 bytes"}
	}

	switch request.Operation {
	case "sessions":
		if request.Session != 0 || request.Code != "" {
			return request, &Error{Operation: "request", Reason: "sessions takes no session or code"}
		}
	case "eval":
		if request.Session == 0 || request.Code == "" || len(request.Code) > 65536 {
			return request, &Error{Operation: "request", Reason: "eval requires a positive session and 1 to 65536 bytes of Lua"}
		}
	case "load":
		if request.Session == 0 || len(request.Code) == 0 || len(request.Code) > 1<<20 {
			return request, &Error{Operation: "request", Reason: "load requires a session and at most 1 MiB of source"}
		}
	case "stop":
		if request.Session == 0 || request.Code != "" {
			return request, &Error{Operation: "request", Reason: "stop requires a session and no source"}
		}
	case "inject":
		if request.Session == 0 || request.Code != "" || request.Body == "" || len(request.Body) > 2*MaxInjectBodyBytes {
			return request, &Error{Operation: "request", Reason: "inject requires a session, a body and no code"}
		}

		if request.PacketCode < -32768 || request.PacketCode > 32767 {
			return request, &Error{Operation: "request", Reason: "packet code must fit a signed 16 bit integer"}
		}

		body, err := hex.DecodeString(request.Body)
		if err != nil {
			return request, &Error{Operation: "request", Reason: "body must be hexadecimal"}
		}

		if len(body) == 0 || len(body) > MaxInjectBodyBytes {
			return request, &Error{Operation: "request", Reason: "body must contain 1 to 60000 bytes"}
		}
	default:
		return request, &Error{Operation: "request", Reason: "unknown operation " + request.Operation}
	}

	return request, nil
}

func failure(id, code string, err error) Response {
	message := err.Error()
	if len(message) > 4096 {
		message = message[:4096] + "... truncated"
	}

	return Response{ID: id, Error: &Failure{Code: code, Message: message}}
}
