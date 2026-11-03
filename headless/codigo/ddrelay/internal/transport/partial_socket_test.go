package transport

import (
	"io"
	"net"
	"time"
)

type partialSocket struct{ written []byte }

func (s *partialSocket) Read([]byte) (int, error) { return 0, io.EOF }

func (s *partialSocket) Write(data []byte) (int, error) {
	s.written = append(s.written, data[:3]...)

	return 3, io.ErrClosedPipe
}

func (s *partialSocket) Close() error { return nil }

func (s *partialSocket) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234}
}

func (s *partialSocket) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 2345}
}

func (s *partialSocket) SetDeadline(time.Time) error { return nil }

func (s *partialSocket) SetReadDeadline(time.Time) error { return nil }

func (s *partialSocket) SetWriteDeadline(time.Time) error { return nil }
