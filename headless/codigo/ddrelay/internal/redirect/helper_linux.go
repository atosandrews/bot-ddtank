package redirect

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func RunHelper() (result error) {
	if os.Geteuid() != 0 {
		return &Error{Operation: "helper", Reason: "root privileges are required"}
	}

	uid := uint64(os.Getuid())
	if value := os.Getenv("SUDO_UID"); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return &Error{Operation: "invoking uid", Cause: err}
		}

		uid = parsed
	}

	connection, err := net.FileConn(os.Stdin)
	if err != nil {
		return &Error{Operation: "helper channel", Cause: err}
	}

	if err := os.Stdin.Close(); err != nil {
		return errors.Join(err, connection.Close())
	}

	channel, ok := connection.(*net.UnixConn)
	if !ok {
		return errors.Join(&Error{Operation: "helper channel", Reason: "Unix socket required"}, connection.Close())
	}

	var closeOnce sync.Once
	var closeErr error
	closeChannel := func() { closeOnce.Do(func() { closeErr = channel.Close() }) }
	defer func() { closeChannel(); result = errors.Join(result, closeErr) }()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()
	stop := context.AfterFunc(ctx, closeChannel)
	defer stop()
	runner, err := NewCommandRunner()
	if err != nil {
		return err
	}

	return serveHelper(ctx, channel, uint32(uid), runner)
}

func serveHelper(ctx context.Context, channel Channel, uid uint32, runner Runner) (result error) {
	var manager *Manager
	defer func() {
		if manager != nil {
			cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			result = errors.Join(result, manager.Close(cleanup))
		}
	}()

	for {
		message, fds, err := receiveMessage(channel)
		if err != nil {
			descriptorErr := closeDescriptors(fds)
			if errors.Is(err, io.EOF) || ctx.Err() != nil {
				return descriptorErr
			}

			return errors.Join(err, descriptorErr)
		}

		operationCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		response, operationErr := helperOperation(operationCtx, message, fds, uid, &manager, runner)
		cancel()
		if operationErr != nil {
			response.Failure = operationErr.Error()
		}

		if err := sendMessage(channel, response, -1); err != nil {
			return errors.Join(err, operationErr)
		}

		if message.Operation == "stop" || operationErr != nil {
			return operationErr
		}
	}
}

func helperOperation(ctx context.Context, message Message, fds []int, uid uint32, manager **Manager, runner Runner) (response Message, result error) {
	defer func() { result = errors.Join(result, closeDescriptors(fds)) }()

	switch message.Operation {
	case "configure":
		if *manager != nil || message.Config == nil || len(fds) != 0 || message.Config.UID != uid {
			return response, &Error{Operation: "configure", Reason: "one configuration for the invoking user is required"}
		}
		var token [16]byte
		if _, err := rand.Read(token[:]); err != nil {
			return response, &Error{Operation: "instance token", Cause: err}
		}
		configured, err := NewManager(*message.Config, hex.EncodeToString(token[:]), runner)
		if err != nil {
			return response, err
		}
		*manager = configured
		response.Table = configured.Table()
		return response, configured.Install(ctx)
	case "mark":
		if *manager == nil || len(fds) != 1 || message.Config != nil {
			return response, &Error{Operation: "mark", Reason: "configured helper and one socket descriptor required"}
		}
		kind, err := unix.GetsockoptInt(fds[0], unix.SOL_SOCKET, unix.SO_TYPE)
		if err != nil || kind != unix.SOCK_STREAM {
			return response, &Error{Operation: "mark", Reason: "TCP stream socket required", Cause: err}
		}
		address, err := unix.Getsockname(fds[0])
		if err != nil {
			return response, &Error{Operation: "mark address", Cause: err}
		}
		if _, ok := address.(*unix.SockaddrInet4); !ok {
			return response, &Error{Operation: "mark", Reason: "IPv4 socket required"}
		}
		if err := unix.SetsockoptInt(fds[0], unix.SOL_SOCKET, unix.SO_MARK, int((*manager).config.Mark)); err != nil {
			return response, &Error{Operation: "mark", Cause: err}
		}
		return response, nil
	case "stop":
		if len(fds) != 0 {
			return response, &Error{Operation: "stop", Reason: "unexpected descriptors"}
		}
		if *manager != nil {
			return response, (*manager).Close(ctx)
		}
		return response, nil
	default:
		return response, &Error{Operation: "helper", Reason: "unsupported operation " + message.Operation}
	}
}
