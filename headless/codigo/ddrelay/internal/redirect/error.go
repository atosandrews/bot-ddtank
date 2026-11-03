package redirect

import "fmt"

type Error struct {
	Operation, Reason string
	Cause             error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("redirect %s: %v", e.Operation, e.Cause)
	}

	return "redirect " + e.Operation + ": " + e.Reason
}

func (e *Error) Unwrap() error { return e.Cause }
