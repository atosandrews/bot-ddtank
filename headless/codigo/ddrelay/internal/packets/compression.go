package packets

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
)

const maxInflatedBody = 4 << 20

func inflate(data []byte) ([]byte, error) {
	source := bytes.NewReader(data)
	stream, err := zlib.NewReader(source)
	if err != nil {
		return nil, err
	}
	plain, readErr := io.ReadAll(io.LimitReader(stream, maxInflatedBody+1))
	err = errors.Join(readErr, stream.Close())
	if err != nil {
		return nil, err
	}
	if len(plain) > maxInflatedBody {
		return nil, fmt.Errorf("%w: inflated body exceeds limit", ErrInvalid)
	}
	if source.Len() != 0 {
		return nil, fmt.Errorf("%w: trailing compressed bytes", ErrInvalid)
	}
	return plain, nil
}
