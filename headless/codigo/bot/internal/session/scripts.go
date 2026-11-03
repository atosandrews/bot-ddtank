package session

import (
	"context"
	"crypto/sha256"

	"google.golang.org/protobuf/proto"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/bot/internal/script"
)

const maxScriptRequests = 4096

func (s *Session) scriptRequest(ctx context.Context, message *pb.RelayMessage) ([]*pb.BotMessage, error) {
	var id, source, kind string

	switch m := message.Payload.(type) {
	case *pb.RelayMessage_Evaluate:
		id, source, kind = m.Evaluate.RequestId, m.Evaluate.Source, "eval"
	case *pb.RelayMessage_Load:
		id, source, kind = m.Load.RequestId, m.Load.Source, "load"
	case *pb.RelayMessage_Stop:
		id, source, kind = m.Stop.RequestId, emptyScript, "stop"
	default:
		return nil, &Error{Code: "invalid_operation"}
	}

	hash := sha256.Sum256([]byte(kind + "\x00" + source))
	if previous, ok := s.requests[id]; ok {
		if previous.hash != hash {
			return s.rejectScriptRequest(id, &Error{Code: "request_conflict", Reason: "request ID already used for different source or operation"}), nil
		}

		if previous.result == nil {
			return s.rejectScriptRequest(id, &Error{Code: "result_expired", Reason: "request was already executed; its result is no longer retained"}), nil
		}

		return []*pb.BotMessage{{Payload: &pb.BotMessage_Result{Result: previous.result}}}, nil
	}

	if len(s.requests) >= maxScriptRequests {
		return s.rejectScriptRequest(id, &Error{Code: "request_limit", Reason: "session request ledger is full"}), nil
	}

	result := &pb.ScriptResult{RequestId: id}
	checkpoint := s.state.LastActionID()
	var value any
	var err error
	if kind == "eval" {
		value, err = s.runtime.Evaluate(ctx, source)
	} else {
		var candidate *script.Runtime
		candidate, err = script.NewRuntimeWithOptions(s.state, []byte(source), s.logs, s.timeout, s.options)
		if err == nil {
			s.state.CancelAfter(0, "script replaced")
			s.runtime.Close()
			s.runtime = candidate
			s.scriptLoaded = kind == "load"
			s.scriptGeneration++
			value = true
		}
	}

	if err == nil {
		result.Value, err = contract.EncodeValue(value)
	}

	for id := checkpoint + 1; id <= s.state.LastActionID(); id++ {
		result.ActionIds = append(result.ActionIds, id)
	}

	result.ScriptGeneration = s.scriptGeneration
	if err == nil && proto.Size(result) > contract.MaxResultBytes {
		err = &Error{Code: "result_limit", Reason: "result exceeds 256 KiB"}
	}

	if err != nil {
		s.state.CancelAfter(checkpoint, "script request failed")
		result.Value = nil
		result.Error = &pb.Failure{Code: pb.FailureCode_FAILURE_CODE_SCRIPT_FAILED, Message: boundedError(err)}
	}

	messages, collectErr := s.collect()
	if collectErr != nil {
		return nil, collectErr
	}

	size := proto.Size(result)
	s.requests[id] = &requestResult{hash: hash, result: result, size: size}
	s.requestOrder = append(s.requestOrder, id)
	s.resultBytes += size

	for s.resultBytes > 2<<20 && len(s.requestOrder) > 0 {
		old := s.requestOrder[0]
		s.requestOrder = s.requestOrder[1:]
		entry := s.requests[old]
		s.resultBytes -= entry.size
		entry.result = nil
		entry.size = 0
	}

	return append(messages, &pb.BotMessage{Payload: &pb.BotMessage_Result{Result: result}}), nil
}

func (s *Session) rejectScriptRequest(id string, err error) []*pb.BotMessage {
	result := &pb.ScriptResult{
		RequestId:        id,
		ScriptGeneration: s.scriptGeneration,
		Error: &pb.Failure{
			Code:    pb.FailureCode_FAILURE_CODE_SCRIPT_FAILED,
			Message: boundedError(err),
		},
	}

	return []*pb.BotMessage{{Payload: &pb.BotMessage_Result{Result: result}}}
}

func boundedError(err error) string {
	message := err.Error()
	if len(message) > 4096 {
		return "script failed: error message exceeds limit"
	}

	return message
}
