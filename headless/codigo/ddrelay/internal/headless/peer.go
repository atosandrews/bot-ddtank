package headless

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"net"
	"time"

	"ddbot/relay/internal/framing"
)

const (
	loginTimeout = 15 * time.Second
	readTimeout  = 90 * time.Second
	writeTimeout = 10 * time.Second
)

type Peer struct {
	authenticator *Authenticator
	dialer        *net.Dialer
	logger        *slog.Logger
}

func NewPeer(authenticator *Authenticator, dialer *net.Dialer, logger *slog.Logger) *Peer {
	return &Peer{authenticator: authenticator, dialer: dialer, logger: logger}
}

func (p *Peer) Run(ctx context.Context, ticket Ticket) (result error) {
	return p.RunWithReady(ctx, ticket, nil)
}

// RunWithReady reports readiness after the game accepts this character and the first heartbeat is written.
func (p *Peer) RunWithReady(ctx context.Context, ticket Ticket, ready func()) (result error) {
	connection, err := p.dialer.DialContext(ctx, "tcp4", ticket.Character.Upstream)
	if err != nil {
		return &Error{Operation: "connect", Reason: "game connection failed", Cause: err}
	}

	stop := context.AfterFunc(ctx, func() {
		if err := connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			p.logger.Error("headless close failed", "character_id", ticket.Character.ID)
		}
	})
	defer stop()
	defer func() {
		if err := connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			result = errors.Join(result, &Error{Operation: "close", Reason: "game connection close failed", Cause: err})
		}
	}()

	var key [8]byte
	if _, err := rand.Read(key[:]); err != nil {
		return &Error{Operation: "login", Reason: "cannot generate cipher key", Cause: err}
	}

	wire, err := p.loginFrame(ticket, key)
	if err != nil {
		return err
	}

	if err := writeWire(connection, wire); err != nil {
		return err
	}

	incoming := framing.NewSessionFramer(key)
	outgoing := framing.NewSessionCipher(key)
	buffer := make([]byte, 32768)
	confirmed := false
	loginDeadline := time.Now().Add(loginTimeout)
	heartbeat := func() error {
		response := framing.Frame{Header: framing.Header{Code: 4}, Body: append(timestamp(time.Now()), 0)}
		encoded, err := outgoing.Encode(response)
		if err != nil {
			return &Error{Operation: "heartbeat", Reason: "cannot encode response", Cause: err}
		}

		return writeWire(connection, encoded)
	}

	for {
		deadline := loginDeadline
		if confirmed {
			deadline = time.Now().Add(readTimeout)
		}

		if err := connection.SetReadDeadline(deadline); err != nil {
			return &Error{Operation: "read", Reason: "cannot set game deadline", Cause: err}
		}

		n, readErr := connection.Read(buffer)
		if ctx.Err() != nil {
			return nil
		}

		for _, segment := range incoming.Push(buffer[:n]) {
			if segment.Kind != framing.FrameSegment || !segment.Decrypted {
				return &Error{Operation: "read", Reason: "invalid encrypted game frame"}
			}

			frame := segment.Frame
			switch frame.Header.Code {
			case 1:
				if confirmed || len(frame.Body) == 0 || frame.Body[0] != 0 || frame.Header.ClientID != ticket.Character.ID {
					return &Error{Operation: "login", Reason: "game rejected login or returned another character"}
				}

				confirmed = true
				p.logger.Info("headless game login confirmed", "character_id", ticket.Character.ID, "name", ticket.Character.Name)
				// A relay in front of this peer learns the session key from the first frame after
				// login and cannot inject commands before it; send one right away instead of
				// waiting for the server's first heartbeat.
				if err := heartbeat(); err != nil {
					return err
				}
				if ready != nil {
					ready()
				}
			case 4:
				if err := heartbeat(); err != nil {
					return err
				}
			default:
				continue
			}
		}

		if readErr != nil {
			return &Error{Operation: "read", Reason: "game connection ended", Cause: readErr}
		}
	}
}

func (p *Peer) loginFrame(ticket Ticket, key [8]byte) ([]byte, error) {
	plain := append(timestamp(time.Now().UTC()), key[:]...)
	plain = append(plain, []byte(ticket.User+","+ticket.Password)...)
	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, p.authenticator.key, plain)
	if err != nil {
		return nil, &Error{Operation: "login", Reason: "cannot encode socket credentials", Cause: err}
	}

	body := make([]byte, 8, 8+len(encrypted))
	binary.BigEndian.PutUint32(body, clientBuild)
	body = append(body, encrypted...)

	return framing.EncodeLogin(framing.Frame{Header: framing.Header{Code: 1}, Body: body})
}

func writeWire(connection net.Conn, data []byte) error {
	if err := connection.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return &Error{Operation: "write", Reason: "cannot set game deadline", Cause: err}
	}

	for len(data) > 0 {
		n, err := connection.Write(data)
		data = data[n:]
		if err != nil {
			return &Error{Operation: "write", Reason: "game write failed; not retried", Cause: err}
		}

		if n == 0 {
			return &Error{Operation: "write", Reason: "incomplete game write; not retried", Cause: io.ErrShortWrite}
		}
	}

	return nil
}
