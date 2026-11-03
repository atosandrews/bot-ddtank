package redirect

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type Client struct {
	mu      sync.Mutex
	channel *net.UnixConn
	table   string
	closed  bool
	done    chan struct{}
	waitErr error
}

func Start(ctx context.Context, config Config, stderr io.Writer) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	executable, err := os.Executable()
	if err != nil {
		return nil, &Error{Operation: "executable", Cause: err}
	}

	pair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_SEQPACKET|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, &Error{Operation: "helper socketpair", Cause: err}
	}

	parent, child := os.NewFile(uintptr(pair[0]), "redirect-parent"), os.NewFile(uintptr(pair[1]), "redirect-child")
	connection, err := net.FileConn(parent)
	parentErr := parent.Close()
	if err != nil || parentErr != nil {
		if connection != nil {
			parentErr = errors.Join(parentErr, connection.Close())
		}

		return nil, errors.Join(err, parentErr, child.Close())
	}

	channel, ok := connection.(*net.UnixConn)
	if !ok {
		return nil, errors.Join(&Error{Operation: "helper channel", Reason: "Unix socket required"}, connection.Close(), child.Close())
	}

	command := exec.Command(executable, "__redirect-helper")
	if os.Geteuid() != 0 {
		command = exec.Command("sudo", "--", executable, "__redirect-helper")
	}

	command.Stdin = child
	command.Stdout = io.Discard
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		return nil, errors.Join(&Error{Operation: "start helper", Cause: err}, child.Close(), channel.Close())
	}

	client := &Client{channel: channel, done: make(chan struct{})}
	go func() { client.waitErr = command.Wait(); close(client.done) }()
	if err := child.Close(); err != nil {
		return nil, errors.Join(err, client.abort())
	}

	response, err := client.exchange(ctx, Message{Operation: "configure", Config: &config}, -1)
	client.table = response.Table
	if err != nil {
		return nil, errors.Join(err, client.abort())
	}

	return client, nil
}

func (c *Client) Table() string { return c.table }

func (c *Client) exchange(ctx context.Context, message Message, fd int) (Message, error) {
	deadline := time.Now().Add(10 * time.Second)
	if end, ok := ctx.Deadline(); ok {
		deadline = end
	}

	if err := c.channel.SetDeadline(deadline); err != nil {
		return Message{}, &Error{Operation: "helper deadline", Cause: err}
	}

	if err := sendMessage(c.channel, message, fd); err != nil {
		return Message{}, err
	}

	response, fds, err := receiveMessage(c.channel)
	if descriptorErr := closeDescriptors(fds); descriptorErr != nil {
		err = errors.Join(err, descriptorErr)
	}

	if err != nil {
		return response, &Error{Operation: "helper reply", Cause: err}
	}

	if len(fds) != 0 {
		return response, &Error{Operation: "helper reply", Reason: "unexpected descriptors"}
	}

	if response.Failure != "" {
		return response, &Error{Operation: "helper", Reason: response.Failure}
	}

	return response, nil
}

func (c *Client) Control(network, address string, raw syscall.RawConn) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return &Error{Operation: "mark", Reason: "helper closed"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var markErr error
	if err := raw.Control(func(fd uintptr) { _, markErr = c.exchange(ctx, Message{Operation: "mark"}, int(fd)) }); err != nil {
		return &Error{Operation: "mark raw socket", Cause: err}
	}

	return markErr
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}

	c.closed = true
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	_, err := c.exchange(ctx, Message{Operation: "stop"}, -1)
	cleanupErr := c.abort()
	if cleanupErr == nil {
		return nil
	}

	return errors.Join(err, cleanupErr)
}

func (c *Client) abort() error {
	closeErr := c.channel.Close()

	select {
	case <-c.done:
		return errors.Join(closeErr, c.waitErr)
	case <-time.After(15 * time.Second):
		reason := "helper still running; cancel any pending sudo authentication and inspect nftables for ddtank tables"
		if c.table != "" {
			reason = "helper still running; inspect and remove table " + c.table
		}
		return errors.Join(closeErr, &Error{Operation: "helper shutdown", Reason: reason})
	}
}

func (c *Client) Done() <-chan struct{} { return c.done }
