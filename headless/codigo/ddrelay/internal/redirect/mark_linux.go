package redirect

import (
	"syscall"

	"golang.org/x/sys/unix"
)

func MarkSocket(mark uint32) func(string, string, syscall.RawConn) error {
	return func(network, address string, raw syscall.RawConn) error {
		var markErr error
		if err := raw.Control(func(fd uintptr) { markErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_MARK, int(mark)) }); err != nil {
			return &Error{Operation: "socket control", Cause: err}
		}

		if markErr != nil {
			return &Error{Operation: "mark upstream socket", Cause: markErr}
		}

		return nil
	}
}
