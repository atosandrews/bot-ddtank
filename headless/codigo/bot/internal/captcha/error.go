package captcha

import "fmt"

type Error struct {
	Operation string
	Reason    string
	Cause     error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("captcha %s: %s: %v", e.Operation, e.Reason, e.Cause)
	}

	return fmt.Sprintf("captcha %s: %s", e.Operation, e.Reason)
}

func (e *Error) Unwrap() error { return e.Cause }
