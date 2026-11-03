package app

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/fx"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/relay/internal/control"
	"ddbot/relay/internal/link"
	"ddbot/relay/internal/packets"
	"ddbot/relay/internal/redirect"
	"ddbot/relay/internal/transport"
)

const proxySocketMark = 0xdd7a

type proxyRuntime struct {
	client        pb.BotClient
	instance      string
	config        ProxyConfig
	recorder      Recorder
	bodies        *packets.Decoder
	logger        *slog.Logger
	source        []byte
	registry      *control.Registry
	control       *control.Server
	helper        *redirect.Client
	listener      net.Listener
	cancel        context.CancelFunc
	controlCancel context.CancelFunc
	done          chan struct{}
	controlDone   chan struct{}
	stopping      chan struct{}
	mu            sync.Mutex
	err           error
}

func newProxyRuntime(config ProxyConfig, recorder Recorder, bodies *packets.Decoder, logger *slog.Logger, source []byte, registry *control.Registry, client pb.BotClient) *proxyRuntime {
	runtime := &proxyRuntime{
		client:      client,
		instance:    rand.Text(),
		config:      config,
		recorder:    recorder,
		bodies:      bodies,
		logger:      logger,
		source:      source,
		registry:    registry,
		done:        make(chan struct{}),
		controlDone: make(chan struct{}),
		stopping:    make(chan struct{}),
	}
	if config.ControlPath != "" {
		runtime.control = control.NewServer(config.ControlPath, registry, runtime.reportControl)
	}

	return runtime
}

func (r *proxyRuntime) start(startCtx context.Context, shutdown fx.Shutdowner) (result error) {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	controlCtx, controlCancel := context.WithCancel(context.Background())
	r.controlCancel = controlCancel
	defer func() {
		if result != nil {
			if r.helper != nil {
				result = errors.Join(result, r.helper.Close())
			}

			cancel()
			controlCancel()
			if r.control != nil {
				result = errors.Join(result, r.control.Close())
			}

			if r.listener != nil {
				result = errors.Join(result, r.listener.Close())
			}
		}
	}()

	network := "tcp"
	if r.config.Transparent {
		network = "tcp4"
	}

	listener, err := net.Listen(network, r.config.Listen)
	if err != nil {
		return &transport.Error{Operation: "listen", Cause: err}
	}

	r.listener = listener
	if r.control != nil {
		if err := r.control.Open(); err != nil {
			return err
		}
	}

	if r.config.ManageRedirect {
		_, port, err := net.SplitHostPort(listener.Addr().String())
		if err != nil {
			return err
		}

		number, err := strconv.ParseUint(port, 10, 16)
		if err != nil {
			return err
		}

		helper, err := redirect.Start(startCtx, redirect.Config{
			UID:   uint32(os.Getuid()),
			Port:  uint16(number),
			Ports: r.config.Ports,
			Mark:  proxySocketMark,
		}, os.Stderr)
		if err != nil {
			return err
		}

		r.helper = helper
	}

	mode := r.config.Upstream
	if r.config.Transparent {
		mode = fmt.Sprintf("original IPv4 destination, game ports %v", r.config.Ports)
	}

	r.logger.InfoContext(startCtx, "relay listening", "address", listener.Addr().String(), "upstream", mode, "control", r.config.ControlPath, "recording", r.recorder.Destination())
	if r.helper != nil {
		r.logger.InfoContext(startCtx, "redirect ready", "table", r.helper.Table())
		go func() {
			select {
			case <-r.helper.Done():
				select {
				case <-r.stopping:
					return
				default:
					r.fail(&redirect.Error{Operation: "helper", Reason: "helper exited; proxy stopping"})
					r.cancel()
				}
			case <-r.stopping:
			}
		}()
	}

	go func() {
		defer close(r.controlDone)
		if r.control != nil {
			if err := r.control.Serve(controlCtx); err != nil {
				r.fail(err)
				r.cancel()
			}
		}
	}()
	go func() {
		defer close(r.done)
		r.run(ctx)
		if err := shutdown.Shutdown(); err != nil {
			r.fail(err)
		}
	}()

	return nil
}

func (r *proxyRuntime) run(ctx context.Context) {
	stop := context.AfterFunc(ctx, func() {
		if err := r.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			r.fail(err)
		}
	})
	defer stop()
	var connections sync.WaitGroup
	defer connections.Wait()
	slots := make(chan struct{}, r.config.MaxSessions)
	var stream uint64

	for {
		client, err := r.listener.Accept()
		if err != nil {
			if ctx.Err() == nil {
				r.fail(&transport.Error{Operation: "accept", Cause: err})
				r.cancel()
			}

			return
		}

		select {
		case slots <- struct{}{}:
		default:
			if err := client.Close(); err != nil {
				r.report(0, err)
			}
			continue
		}

		stream++
		id := stream
		connections.Add(1)
		go func() {
			defer connections.Done()
			defer func() { <-slots }()
			upstream, err := r.dial(ctx, client)
			if err != nil {
				closeErr := client.Close()
				if ctx.Err() == nil {
					r.report(id, errors.Join(err, closeErr))
				} else if closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
					r.fail(closeErr)
				}

				return
			}

			connection := transport.NewConnection(id, client, upstream, r.recorder, nil, r.bodies, func(checkpoint func(context.Context) (link.Checkpoint, error)) transport.Automation {
				return link.New(r.client, r.config.APIKey, &pb.OpenSession{
					ApiVersion:      contract.APIVersion,
					CodecRevision:   contract.CodecRevision,
					RelayInstanceId: r.instance,
					GameSessionId:   rand.Text(),
					RelayId:         r.config.RelayID,
					LocalStream:     id,
				}, string(r.source), r.logger, checkpoint)
			})
			r.registry.Add(id, connection)
			defer r.registry.Remove(id)
			if err := connection.Run(ctx); err != nil {
				var transportErr *transport.Error
				if errors.As(err, &transportErr) && strings.HasPrefix(transportErr.Operation, "record") {
					r.fail(err)
					r.cancel()

					return
				}

				if ctx.Err() == nil {
					r.report(id, err)
				}
			}
		}()
	}
}

func (r *proxyRuntime) dial(ctx context.Context, client net.Conn) (net.Conn, error) {
	address := r.config.Upstream
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	network := "tcp"
	if r.config.Transparent {
		resolved, err := redirect.OriginalDestination(client, r.config.Ports)
		if err != nil {
			return nil, err
		}

		address = resolved
		network = "tcp4"
		if r.helper != nil {
			dialer.Control = r.helper.Control
		} else {
			dialer.Control = redirect.MarkSocket(proxySocketMark)
		}
	}

	upstream, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, &transport.Error{Operation: "dial", Cause: err}
	}

	return upstream, nil
}

func (r *proxyRuntime) report(id uint64, err error) {
	r.logger.Error("relay session failed", "session", id, "error", err)
}

func (r *proxyRuntime) reportControl(err error) { r.report(0, err) }

func (r *proxyRuntime) fail(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = errors.Join(r.err, err)
}

func (r *proxyRuntime) stop() error {
	close(r.stopping)
	r.controlCancel()
	var cleanupErr error
	if r.control != nil {
		cleanupErr = r.control.Close()
	}

	if r.helper != nil {
		cleanupErr = errors.Join(cleanupErr, r.helper.Close())
	}

	r.cancel()
	<-r.done
	<-r.controlDone
	r.mu.Lock()
	defer r.mu.Unlock()

	return errors.Join(r.err, cleanupErr)
}
