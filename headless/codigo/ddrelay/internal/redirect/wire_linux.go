package redirect

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"

	"golang.org/x/sys/unix"
)

type Channel interface {
	ReadMsgUnix([]byte, []byte) (int, int, int, *net.UnixAddr, error)
	WriteMsgUnix([]byte, []byte, *net.UnixAddr) (int, int, error)
}

type Message struct {
	Operation string  `json:"operation,omitempty"`
	Config    *Config `json:"config,omitempty"`
	Table     string  `json:"table,omitempty"`
	Failure   string  `json:"failure,omitempty"`
}

func sendMessage(connection Channel, message Message, fd int) error {
	data, err := json.Marshal(message)
	if err != nil {
		return &Error{Operation: "encode helper message", Cause: err}
	}

	var rights []byte
	if fd >= 0 {
		rights = unix.UnixRights(fd)
	}

	n, oob, err := connection.WriteMsgUnix(data, rights, nil)
	if err != nil {
		return &Error{Operation: "send helper message", Cause: err}
	}

	if n != len(data) || oob != len(rights) {
		return &Error{Operation: "send helper message", Cause: io.ErrShortWrite}
	}

	return nil
}

func receiveMessage(connection Channel) (Message, []int, error) {
	data, rights := make([]byte, 4096), make([]byte, unix.CmsgSpace(4*4))
	n, oob, flags, _, err := connection.ReadMsgUnix(data, rights)
	if err != nil {
		return Message{}, nil, err
	}

	if n == 0 {
		return Message{}, nil, io.EOF
	}

	controls, err := unix.ParseSocketControlMessage(rights[:oob])
	if err != nil {
		return Message{}, nil, &Error{Operation: "helper rights", Cause: err}
	}

	var fds []int

	for _, control := range controls {
		if control.Header.Level != unix.SOL_SOCKET || control.Header.Type != unix.SCM_RIGHTS {
			return Message{}, fds, &Error{Operation: "helper rights", Reason: "unexpected ancillary data"}
		}

		received, err := unix.ParseUnixRights(&control)
		if err != nil {
			return Message{}, fds, &Error{Operation: "helper descriptors", Cause: err}
		}

		for _, fd := range received {
			unix.CloseOnExec(fd)
		}

		fds = append(fds, received...)
	}

	if flags&(unix.MSG_TRUNC|unix.MSG_CTRUNC) != 0 {
		return Message{}, fds, &Error{Operation: "helper message", Reason: "message truncated"}
	}

	var message Message
	decoder := json.NewDecoder(bytes.NewReader(data[:n]))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&message); err != nil {
		return message, fds, &Error{Operation: "helper message", Cause: err}
	}

	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return message, fds, &Error{Operation: "helper message", Reason: "trailing JSON"}
	}

	return message, fds, nil
}

func closeDescriptors(fds []int) error {
	var result error

	for _, fd := range fds {
		result = errors.Join(result, unix.Close(fd))
	}

	return result
}
