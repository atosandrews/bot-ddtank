package commands

import (
	"encoding/binary"
	"math"
	"unicode/utf8"

	"ddbot/relay/internal/framing"
)

const maxBodySize = math.MaxUint16 - framing.HeaderSize

type writer struct {
	data []byte
	err  error
}

func (w *writer) reserve(size int) bool {
	if w.err != nil {
		return false
	}
	if size > maxBodySize-len(w.data) {
		w.err = &EncodingError{Field: "body", Reason: "body exceeds frame limit"}
		return false
	}

	return true
}

func (w *writer) byte(value byte) {
	if w.reserve(1) {
		w.data = append(w.data, value)
	}
}

func (w *writer) ints(values ...int32) {
	if !w.reserve(4 * len(values)) {
		return
	}

	for _, value := range values {
		w.data = binary.BigEndian.AppendUint32(w.data, uint32(value))
	}
}

func (w *writer) int8(field string, value int32) {
	if w.err != nil {
		return
	}
	if value < math.MinInt8 || value > math.MaxInt8 {
		w.err = &EncodingError{Field: field, Reason: "value must fit a signed 8-bit integer"}
		return
	}

	w.byte(byte(value))
}

func (w *writer) int16(field string, value int32) {
	if w.err != nil {
		return
	}
	if value < math.MinInt16 || value > math.MaxInt16 {
		w.err = &EncodingError{Field: field, Reason: "value must fit a signed 16-bit integer"}
		return
	}
	if !w.reserve(2) {
		return
	}

	w.data = binary.BigEndian.AppendUint16(w.data, uint16(value))
}

func (w *writer) flag(value bool) {
	if value {
		w.byte(1)
		return
	}

	w.byte(0)
}

func (w *writer) text(field, value string) {
	if w.err != nil {
		return
	}
	if len(value) > math.MaxUint16 || !utf8.ValidString(value) {
		w.err = &EncodingError{Field: field, Reason: "string must be UTF-8 and fit an unsigned 16-bit length"}
		return
	}
	if !w.reserve(2 + len(value)) {
		return
	}

	w.data = binary.BigEndian.AppendUint16(w.data, uint16(len(value)))
	w.data = append(w.data, value...)
}
