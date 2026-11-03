package transport

import (
	"ddbot/relay/internal/capture"
)

func (c *Connection) finishInput(direction capture.Direction) (bool, error) {
	c.closing = true
	destination := c.client

	switch direction {
	case capture.ClientToServer:
		if err := c.relay.Finish(); err != nil {
			return true, err
		}
		c.clientEOF = true
		destination = c.server
	case capture.ServerToClient:
		c.serverEOF = true
	default:
		return true, &ControlError{Code: "invalid_direction", Reason: string(direction)}
	}

	closer, ok := destination.(interface{ CloseWrite() error })
	if !ok {
		return true, nil
	}

	if err := closer.CloseWrite(); err != nil {
		return true, &Error{Operation: "half-close", Cause: err}
	}

	return c.clientEOF && c.serverEOF, nil
}
