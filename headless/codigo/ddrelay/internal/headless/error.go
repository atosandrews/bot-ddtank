package headless

type Error struct {
	Operation string
	Reason    string
	Cause     error
}

func (e *Error) Error() string {
	return "headless " + e.Operation + ": " + e.Reason
}

func (e *Error) Unwrap() error {
	return e.Cause
}
