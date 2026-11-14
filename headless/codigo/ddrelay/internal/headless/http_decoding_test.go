package headless

import (
	"bytes"
	"io"
	"math/rand"
	"testing"
)

type countedHTTPSource struct {
	io.Reader
	read, closed int
}

func (r *countedHTTPSource) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.read += n
	return n, err
}
func (r *countedHTTPSource) Close() error { r.closed++; return nil }

func TestHTTPDecodingStreamsWithoutReadingWholeEntity(t *testing.T) {
	plain := make([]byte, 512<<10)
	_, _ = rand.New(rand.NewSource(42)).Read(plain)
	for _, encoding := range []string{"gzip", "br", "deflate", "raw-deflate"} {
		t.Run(encoding, func(t *testing.T) {
			wire := encodeHTTP(t, plain, encoding)
			source := &countedHTTPSource{Reader: bytes.NewReader(wire)}
			headerEncoding := encoding
			if encoding == "raw-deflate" {
				headerEncoding = "deflate"
			}
			body, err := decodeHTTPBody(source, headerEncoding)
			if err != nil {
				t.Fatal(err)
			}
			if source.read >= len(wire) {
				t.Error("decoder read entire entity during initialization")
			}
			data := make([]byte, 256)
			if _, err := io.ReadFull(body, data); err != nil || !bytes.Equal(data, plain[:256]) {
				t.Fatalf("streamed prefix changed: %v", err)
			}
			if source.read >= len(wire) {
				t.Error("small read expanded entire entity")
			}
			if err := body.Close(); err != nil {
				t.Fatal(err)
			}
			if source.closed != 1 {
				t.Fatalf("source closed %d times", source.closed)
			}
		})
	}
}

func TestHTTPDecoderInitializationFailureClosesSourceOnce(t *testing.T) {
	for _, tc := range []struct {
		name, encoding string
		data           []byte
	}{
		{"bad gzip", "gzip", []byte("invalid")},
		{"short zlib dictionary", "deflate", []byte{0x78, 0xbb}},
		{"empty deflate", "deflate", nil},
		{"unsupported encoding", "unsupported", []byte("data")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := &countedHTTPSource{Reader: bytes.NewReader(tc.data)}
			if body, err := decodeHTTPBody(source, tc.encoding); err == nil {
				body.Close()
				t.Fatal("invalid compressed body accepted")
			}
			if source.closed != 1 {
				t.Fatalf("source closed %d times", source.closed)
			}
		})
	}
}
