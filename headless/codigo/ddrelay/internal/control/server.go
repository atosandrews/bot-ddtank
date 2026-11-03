package control

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"ddbot/api/contract"
	"ddbot/relay/internal/link"
	"ddbot/relay/internal/transport"
)

type Server struct {
	path     string
	handler  Handler
	report   func(error)
	listener net.Listener
}

func NewServer(path string, handler Handler, report func(error)) *Server {
	return &Server{path: path, handler: handler, report: report}
}

func (s *Server) Open() error {
	directory := filepath.Dir(s.path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return &Error{Operation: "create directory", Cause: err}
	}

	info, err := os.Lstat(directory)
	if err != nil {
		return &Error{Operation: "stat directory", Cause: err}
	}

	owner, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || info.Mode().Perm()&0077 != 0 || int(owner.Uid) != os.Getuid() {
		return &Error{Operation: "directory", Reason: "socket parent must be a private directory owned by the current user"}
	}

	if _, err := os.Lstat(s.path); !os.IsNotExist(err) {
		return &Error{Operation: "socket", Reason: "path already exists or cannot be inspected; no file was removed"}
	}

	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: s.path, Net: "unix"})
	if err != nil {
		return &Error{Operation: "listen", Cause: err}
	}

	if err := os.Chmod(s.path, 0600); err != nil {
		return errors.Join(&Error{Operation: "permissions", Cause: err}, listener.Close())
	}

	s.listener = listener

	return nil
}

func (s *Server) Serve(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	defer workers.Wait()
	defer cancel()
	slots := make(chan struct{}, 16)

	for {
		connection, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				return nil
			}

			return &Error{Operation: "accept", Cause: err}
		}

		select {
		case slots <- struct{}{}:
		default:
			if err := connection.Close(); err != nil {
				return &Error{Operation: "close excess client", Cause: err}
			}
			continue
		}

		workers.Add(1)
		go func() {
			defer workers.Done()
			defer func() { <-slots }()
			if err := s.Handle(ctx, connection); err != nil {
				s.report(err)
			}
		}()
	}
}

func (s *Server) Close() error {
	if s.listener == nil {
		return nil
	}

	return s.listener.Close()
}

func (s *Server) Handle(ctx context.Context, connection net.Conn) (result error) {
	var once sync.Once
	var closeErr error
	closeConnection := func() { once.Do(func() { closeErr = connection.Close() }) }
	stop := context.AfterFunc(ctx, closeConnection)
	defer stop()
	defer func() {
		closeConnection()
		if !errors.Is(closeErr, net.ErrClosed) {
			result = errors.Join(result, closeErr)
		}
	}()
	scanner := bufio.NewScanner(connection)
	scanner.Buffer(make([]byte, 4096), MaxRequestBytes)
	encoder := json.NewEncoder(connection)

	for scanner.Scan() {
		request, err := Decode(scanner.Bytes())
		response := failure(request.ID, "invalid_request", &Error{Operation: "request", Reason: "invalid request"})
		if err != nil {
			response = failure(request.ID, "invalid_request", err)
		} else {
			requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			response = s.execute(requestCtx, request)
			cancel()
		}

		if err := connection.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
			return &Error{Operation: "response deadline", Cause: err}
		}

		if err := encoder.Encode(response); err != nil {
			return &Error{Operation: "response", Cause: err}
		}
	}

	if ctx.Err() != nil {
		return nil
	}

	if err := scanner.Err(); err != nil {
		return &Error{Operation: "read request", Cause: err}
	}

	return nil
}

func (s *Server) execute(ctx context.Context, request Request) Response {
	response := Response{ID: request.ID}
	var err error

	switch request.Operation {
	case "sessions":
		response.Value, err = s.handler.Sessions(ctx)
	case "eval", "load", "stop":
		result, requestErr := s.handler.Request(ctx, request)
		err = requestErr
		if err == nil {
			if result.Error != nil {
				code, codeErr := contract.FailureCodeName(result.Error.Code)
				if codeErr != nil {
					return failure(request.ID, "invalid_result", codeErr)
				}

				return Response{ID: request.ID, Error: &Failure{Code: code, Message: result.Error.Message}, ActionIDs: result.ActionIds}
			}

			response.Value, err = contract.DecodeValue(result.Value)
			response.ActionIDs = result.ActionIds
			response.ScriptGeneration = result.ScriptGeneration
		}
	case "inject":
		var written int
		written, err = s.handler.Inject(ctx, request)
		response.Value = written
	default:
		return failure(request.ID, "invalid_request", &Error{Operation: "request", Reason: "unknown operation"})
	}

	if err != nil {
		code := "request_failed"
		var transportError *transport.ControlError
		var linkError *link.Error
		if errors.As(err, &transportError) {
			code = transportError.Code
		}

		if errors.As(err, &linkError) {
			code = linkError.Code
		}

		return failure(request.ID, code, err)
	}

	response.OK = true

	return response
}
