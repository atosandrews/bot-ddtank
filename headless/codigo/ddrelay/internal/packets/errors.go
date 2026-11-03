package packets

import (
	"errors"
	"fmt"
)

var (
	ErrUnsupported = errors.New("unsupported packet layout")
	ErrInvalid     = errors.New("invalid packet body")
)

type DecodeError struct {
	Field  string
	Offset int
	Err    error
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("body field %s at offset %d: %v", e.Field, e.Offset, e.Err)
}

func (e *DecodeError) Unwrap() error { return e.Err }
