package game

type SessionError struct{ Reason string }

func (e *SessionError) Error() string { return "session: " + e.Reason }

func (e *SessionError) Unwrap() error { return nil }
