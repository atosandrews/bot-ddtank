package packets

import (
	"bytes"
	"compress/zlib"
	"errors"
	"testing"

	"ddbot/relay/internal/capture"
	"ddbot/relay/internal/framing"
)

func TestRemainingUnsupportedAndBounds(t *testing.T) {
	tests := []struct {
		code     int16
		body     []byte
		status   string
		err      error
		consumed int
	}{
		{167, ints(1, 2), "unknown", ErrUnsupported, 0},
		{68, []byte{255, 1}, "partial", ErrUnsupported, 1},
		{107, ints(99, 1), "partial", ErrUnsupported, 4},
		{128, []byte{255, 1}, "partial", ErrUnsupported, 1},
		{129, []byte{255, 1}, "partial", ErrUnsupported, 1},
		{145, []byte{255, 1}, "partial", ErrUnsupported, 1},
		{155, []byte{255, 1}, "partial", ErrUnsupported, 1},
		{209, []byte{255, 1}, "partial", ErrUnsupported, 1},
		{277, []byte{255, 1}, "partial", ErrUnsupported, 1},
		{333, []byte{255, 1}, "partial", ErrUnsupported, 1},
		{343, []byte{255, 1}, "partial", ErrUnsupported, 1},
		{402, []byte{255, 1}, "partial", ErrUnsupported, 1},
		{75, ints(-1), "invalid", ErrInvalid, 4},
		{178, ints(10001), "invalid", ErrInvalid, 4},
		{333, join([]byte{1}, ints(1, 2, -1)), "invalid", ErrInvalid, 13},
		{402, join([]byte{5}, ints(10001)), "invalid", ErrInvalid, 5},
	}
	for _, tt := range tests {
		got := NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: tt.code}, Body: tt.body})
		if got.Status != tt.status || !errors.Is(got.Err, tt.err) || got.Consumed != tt.consumed || !bytes.Equal(got.Remaining, tt.body[tt.consumed:]) {
			t.Fatalf("code %d result = %#v", tt.code, got)
		}
	}
	for _, code := range []int16{64, 74} {
		for _, body := range [][]byte{nil, {0x78, 0xda}, compressed(t, ints(-1)), append(compressed(t, ints(0)), 0)} {
			got := NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: code}, Body: body})
			if got.Status != "invalid" || got.Err == nil {
				t.Fatalf("code %d result = %#v", code, got)
			}
		}
	}
}

func FuzzRemainingBodies(f *testing.F) {
	for _, code := range []int16{1, 3, 4, 13, 49, 68, 71, 72, 75, 92, 94, 107, 108, 128, 129, 145, 155, 168, 178, 179, 209, 218, 227, 239, 259, 277, 333, 343, 402, 443, 448} {
		f.Add(code, []byte{1, 0, 0, 0, 1, 0, 0, 0, 0})
	}
	for _, code := range []int16{64, 74} {
		var out bytes.Buffer
		w := zlib.NewWriter(&out)
		if _, err := w.Write(ints(0, 0)); err != nil {
			f.Fatal(err)
		}
		if err := w.Close(); err != nil {
			f.Fatal(err)
		}
		f.Add(code, out.Bytes())
	}
	f.Fuzz(func(t *testing.T, code int16, body []byte) {
		if len(body) > 65535 {
			return
		}
		original := bytes.Clone(body)
		for _, direction := range []capture.Direction{capture.ClientToServer, capture.ServerToClient} {
			got := NewDecoder().Decode(direction, framing.Frame{Header: framing.Header{Code: code}, Body: body})
			if got.Consumed < 0 || got.Consumed > got.Total || got.Total-got.Consumed != len(got.Remaining) {
				t.Fatal("invalid byte accounting")
			}
			if got.Status == "decoded" && (got.Err != nil || len(got.Remaining) != 0) {
				t.Fatal("invalid completion status")
			}
			if got.Compression == "" && !bytes.Equal(got.Remaining, body[got.Consumed:]) {
				t.Fatal("unread bytes lost")
			}
			if !bytes.Equal(original, body) {
				t.Fatal("input mutated")
			}
		}
	})
}
