package redirect

import (
	"encoding/binary"
	"net"
	"strconv"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

func OriginalDestination(connection net.Conn, ports []uint16) (string, error) {
	socket, ok := connection.(syscall.Conn)
	if !ok {
		return "", &Error{Operation: "original destination", Reason: "TCP socket required"}
	}

	raw, err := socket.SyscallConn()
	if err != nil {
		return "", &Error{Operation: "socket", Cause: err}
	}

	var data [unix.SizeofSockaddrInet4]byte
	size := uint32(len(data))
	var optionErr error
	if err := raw.Control(func(fd uintptr) {
		_, _, errno := unix.Syscall6(unix.SYS_GETSOCKOPT, fd, unix.SOL_IP, unix.SO_ORIGINAL_DST, uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(&size)), 0)
		if errno != 0 {
			optionErr = errno
		}
	}); err != nil {
		return "", &Error{Operation: "original destination", Cause: err}
	}

	if optionErr != nil {
		return "", &Error{Operation: "original destination", Cause: optionErr}
	}

	if size != uint32(len(data)) {
		return "", &Error{Operation: "original destination", Reason: "invalid sockaddr length"}
	}

	return decodeDestination(data[:], ports)
}

func decodeDestination(data []byte, ports []uint16) (string, error) {
	if len(data) != unix.SizeofSockaddrInet4 || binary.NativeEndian.Uint16(data) != unix.AF_INET {
		return "", &Error{Operation: "original destination", Reason: "IPv4 sockaddr required"}
	}

	ip := net.IP(data[4:8])
	port := binary.BigEndian.Uint16(data[2:4])
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() {
		return "", &Error{Operation: "original destination", Reason: "destination is not a redirected game endpoint"}
	}

	for _, allowed := range ports {
		if allowed == port {
			return net.JoinHostPort(ip.String(), strconv.Itoa(int(port))), nil
		}
	}

	return "", &Error{Operation: "original destination", Reason: "port is outside the redirect scope"}
}
