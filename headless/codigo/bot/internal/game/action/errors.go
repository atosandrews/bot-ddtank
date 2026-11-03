package action

type UnavailableError struct{ Reason string }

func (e *UnavailableError) Error() string { return "command unavailable: " + e.Reason }

func (e *UnavailableError) Unwrap() error { return nil }
