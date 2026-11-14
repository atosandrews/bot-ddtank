package headless

import (
	"bufio"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"io"

	"github.com/andybalholm/brotli"
)

type decodedHTTPBody struct {
	io.Reader
	decoder io.Closer
	source  io.ReadCloser
}

// decodeHTTPBody has already closed the source when initialization fails.
type failedHTTPBody struct{ err error }

func (r failedHTTPBody) Read([]byte) (int, error) { return 0, r.err }
func (r failedHTTPBody) Close() error             { return nil }

func (r *decodedHTTPBody) Close() error {
	var err error
	if r.decoder != nil {
		err = r.decoder.Close()
	}
	return errors.Join(err, r.source.Close())
}

// Stream the HTTP entity into Authenticator's size-limited reader. Do not
// buffer a deflate body before its expanded size can be checked. Peek retains
// both prefix bytes for raw deflate, and failed decoder constructors close only
// the actual source; a nil decoder must never be closed.
func decodeHTTPBody(source io.ReadCloser, encoding string) (io.ReadCloser, error) {
	result := &decodedHTTPBody{source: source}
	var err error
	switch encoding {
	case "gzip":
		var reader *gzip.Reader
		reader, err = gzip.NewReader(source)
		if err == nil {
			result.Reader, result.decoder = reader, reader
		}
	case "deflate":
		buffer := bufio.NewReader(source)
		var prefix []byte
		prefix, err = buffer.Peek(2)
		if err == nil {
			if prefix[0]&0x0f == 8 && (int(prefix[0])<<8|int(prefix[1]))%31 == 0 {
				var reader io.ReadCloser
				reader, err = zlib.NewReader(buffer)
				if err == nil {
					result.Reader, result.decoder = reader, reader
				}
			} else {
				reader := flate.NewReader(buffer)
				result.Reader, result.decoder = reader, reader
			}
		}
	case "br":
		result.Reader = brotli.NewReader(source)
	default:
		err = errors.New("unsupported HTTP content encoding")
	}
	if err != nil {
		closeErr := source.Close()
		return nil, &Error{Operation: "request", Kind: "protocol", Reason: "cannot decode HTTP response", Cause: errors.Join(err, closeErr)}
	}
	return result, nil
}
