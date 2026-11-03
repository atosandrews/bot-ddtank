package framing

type RelayError struct{ Reason string }

func (e *RelayError) Error() string { return "DDTank relay: " + e.Reason }
