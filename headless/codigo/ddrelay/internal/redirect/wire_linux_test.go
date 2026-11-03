package redirect

import (
	"context"
	"errors"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

const proxyTestMark = 0xdd7a

func helperPair(t *testing.T) (*net.UnixConn, *net.UnixConn) {
	t.Helper()
	pair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_SEQPACKET|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}

	files := []*os.File{os.NewFile(uintptr(pair[0]), "test-helper"), os.NewFile(uintptr(pair[1]), "test-helper")}

	for _, file := range files {
		t.Cleanup(func() {
			if err := file.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
				t.Error(err)
			}
		})
	}

	connections := make([]*net.UnixConn, 0, 2)

	for _, file := range files {
		connection, err := net.FileConn(file)
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
				t.Skipf("Unix descriptor operations unavailable: %v", err)
			}

			t.Fatal(errors.Join(err, closeErr))
		}

		channel := connection.(*net.UnixConn)
		if err := channel.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatal(err)
		}

		connections = append(connections, channel)
		t.Cleanup(func() {
			if err := channel.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Error(err)
			}
		})
	}

	return connections[0], connections[1]
}

func TestHelperRemovesRedirectWhenParentChannelCloses(t *testing.T) {
	parent, child := helperPair(t)
	runner := &fakeRunner{}
	done := make(chan error, 1)
	go func() { done <- serveHelper(context.Background(), child, 1000, runner) }()
	config := Config{
		UID:   1000,
		Port:  11401,
		Ports: []uint16{1401},
		Mark:  proxyTestMark,
	}
	if err := sendMessage(parent, Message{Operation: "configure", Config: &config}, -1); err != nil {
		t.Fatal(err)
	}

	response, fds, err := receiveMessage(parent)
	if err != nil || response.Failure != "" || response.Table == "" || len(fds) != 0 {
		t.Fatalf("configure = %#v, fds = %v, err = %v", response, fds, err)
	}

	if err := parent.Close(); err != nil {
		t.Fatal(err)
	}

	if err := <-done; err != nil {
		t.Fatal(err)
	}

	if runner.present || len(runner.inputs) != 2 || runner.inputs[1] != "delete table ip "+response.Table+"\n" {
		t.Fatalf("cleanup = %#v", runner.inputs)
	}
}

func TestHelperRightsAreReceivedAndClosedOnInvalidMark(t *testing.T) {
	parent, child := helperPair(t)
	file, err := os.Open("/dev/null")
	if err != nil {
		t.Fatal(err)
	}

	defer file.Close()
	if err := sendMessage(parent, Message{Operation: "mark"}, int(file.Fd())); err != nil {
		t.Fatal(err)
	}

	message, fds, err := receiveMessage(child)
	if err != nil || len(fds) != 1 {
		t.Fatalf("rights = %v, err = %v", fds, err)
	}

	manager, err := NewManager(Config{
		UID:   1000,
		Port:  11401,
		Ports: []uint16{1401},
		Mark:  proxyTestMark,
	}, "00000000000000000000000000000000", &fakeRunner{})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := helperOperation(context.Background(), message, fds, 1000, &manager, &fakeRunner{}); err == nil {
		t.Fatal("accepted non-socket descriptor")
	}

	if err := unix.Fstat(fds[0], new(unix.Stat_t)); !errors.Is(err, unix.EBADF) {
		t.Fatalf("descriptor remains open: %v", err)
	}
}
