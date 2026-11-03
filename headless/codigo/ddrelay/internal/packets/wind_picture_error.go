package packets

import "fmt"

type WindPictureError struct {
	Reason string
	Cause  error
}

func (e *WindPictureError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("wind picture %s: %v", e.Reason, e.Cause)
	}
	return "wind picture " + e.Reason
}

func (e *WindPictureError) Unwrap() error { return e.Cause }
