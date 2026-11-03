package packets

import (
	"bytes"
	"errors"

	"ddbot/relay/internal/capture"
	"ddbot/relay/internal/framing"
)

type Decoder struct{}

func NewDecoder() *Decoder { return &Decoder{} }

func (d *Decoder) Decode(direction capture.Direction, frame framing.Frame) Result {
	result := Result{Name: packetName(direction, frame.Header.Code), Fields: make(map[string]any)}
	body := frame.Body
	if direction == capture.ServerToClient && (frame.Header.Code == 64 || frame.Header.Code == 74 || frame.Header.Code == 405) {
		plain, err := inflate(body)
		if err != nil {
			result.Status = "invalid"
			result.Err = &DecodeError{Field: "zlib", Err: err}
			result.Total = len(body)
			result.Remaining = bytes.Clone(body)
			return result
		}
		body = plain
		result.Compression = "zlib"
	}
	r := &reader{data: body}
	var err error
	switch direction {
	case capture.ClientToServer:
		err = decodeClient(r, frame.Header, result.Fields)
	case capture.ServerToClient:
		err = decodeServer(r, frame.Header, result.Fields)
	default:
		err = ErrUnsupported
	}
	if r.err != nil {
		err = r.err
	}
	result.Consumed = r.offset
	result.Total = len(body)
	result.Remaining = bytes.Clone(body[r.offset:])
	result.Err = err
	result.Status = "decoded"
	if errors.Is(err, ErrUnsupported) {
		result.Status = "unknown"
		if r.offset > 0 || len(result.Fields) > 0 {
			result.Status = "partial"
		}
	} else if err != nil {
		result.Status = "invalid"
	} else if len(result.Remaining) > 0 {
		result.Status = "partial"
	}
	return result
}
