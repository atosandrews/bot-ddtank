package headless

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"ddbot/relay/internal/framing"
)

// Readiness must follow the accepted character ID and first heartbeat, not just TCP connect.
func TestPeerReportsReadyOnlyForAcceptedCharacter(t *testing.T) {
	for _, acceptedID := range []int32{123, 456} {
		t.Run(strconv.Itoa(int(acceptedID)), func(t *testing.T) {
			key, err := rsa.GenerateKey(rand.Reader, 1024)
			if err != nil {
				t.Fatal(err)
			}
			auth, err := NewAuthenticator(&http.Client{})
			if err != nil {
				t.Fatal(err)
			}
			auth.key = &key.PublicKey
			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			serverDone := make(chan error, 1)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					serverDone <- err
					return
				}
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
				login, err := readOfflineFrame(conn, framing.NewEncryptedFramer())
				if err != nil {
					serverDone <- err
					return
				}
				if len(login.Body) < 8 {
					serverDone <- errors.New("missing RSA payload")
					return
				}
				plain, err := rsa.DecryptPKCS1v15(rand.Reader, key, login.Body[8:])
				if err != nil {
					serverDone <- err
					return
				}
				if len(plain) < 15 || string(plain[15:]) != "fixture-user,fixture-password" {
					serverDone <- errors.New("invalid game credentials")
					return
				}
				var sessionKey [8]byte
				copy(sessionKey[:], plain[7:15])
				response, err := framing.NewSessionCipher(sessionKey).Encode(framing.Frame{Header: framing.Header{Code: 1, ClientID: acceptedID}, Body: []byte{0}})
				if err != nil {
					serverDone <- err
					return
				}
				if _, err = conn.Write(response); err != nil {
					serverDone <- err
					return
				}
				if acceptedID == 123 {
					heartbeat, err := readOfflineFrame(conn, framing.NewSessionFramer(sessionKey))
					if err != nil {
						serverDone <- err
						return
					}
					if heartbeat.Header.Code != 4 {
						serverDone <- errors.New("first heartbeat missing")
						return
					}
					cancel()
				}
				serverDone <- nil
			}()
			var ready atomic.Bool
			peer := NewPeer(auth, &net.Dialer{Timeout: time.Second}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			err = peer.RunWithReady(ctx, Ticket{Character: Character{ID: 123, Upstream: listener.Addr().String()}, User: "fixture-user", Password: "fixture-password"}, func() { ready.Store(true) })
			if serverErr := <-serverDone; serverErr != nil {
				t.Fatal(serverErr)
			}
			if acceptedID == 123 && !ready.Load() {
				t.Fatal("accepted login and heartbeat did not report readiness")
			}
			if acceptedID != 123 && (ready.Load() || err == nil) {
				t.Fatal("another character was reported ready")
			}
		})
	}
}

func readOfflineFrame(conn net.Conn, framer *framing.EncryptedFramer) (framing.Frame, error) {
	buffer := make([]byte, 4096)
	for {
		n, err := conn.Read(buffer)
		for _, segment := range framer.Push(buffer[:n]) {
			if segment.Kind == framing.FrameSegment && segment.Decrypted {
				return segment.Frame, nil
			}
		}
		if err != nil {
			return framing.Frame{}, err
		}
	}
}
