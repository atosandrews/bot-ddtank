package headless

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	fhttp "github.com/bogdanfinn/fhttp"
	tlsclient "github.com/bogdanfinn/tls-client"
)

func TestLauncherTransportPreservesGameBytesAcrossHTTPCompression(t *testing.T) {
	for _, h2 := range []bool{false, true} {
		for _, encoding := range []string{"identity", "gzip", "br", "deflate"} {
			t.Run(fmt.Sprintf("h2=%v/%s", h2, encoding), func(t *testing.T) {
				plain := []byte(`<Result value="true"><Item ID="123" /></Result>`)
				gameBytes := obfuscatedLogin(t, plain)
				wire := encodeHTTP(t, gameBytes, encoding)
				server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.UserAgent() != launcherAgent || r.Header.Get("Accept-Encoding") != "gzip, deflate, br" {
						t.Error("candidate HTTP headers changed")
					}
					w.Header().Set("Content-Encoding", encoding)
					w.Header().Set("CF-Ray", "fixture-ray-GRU")
					_, _ = w.Write(wire)
				}))
				server.EnableHTTP2 = h2
				var classicHello atomic.Bool
				server.TLS = &tls.Config{GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
					// Chrome87 advertises TLS1.0/1.1 as well as TLS1.2/1.3 and
					// a GREASE suite before AES128. A default modern Go handshake
					// must not silently replace the configured compatibility profile.
					if len(hello.SupportedVersions) == 5 && len(hello.CipherSuites) > 1 && hello.CipherSuites[1] == tls.TLS_AES_128_GCM_SHA256 {
						classicHello.Store(true)
					}
					return nil, nil
				}}
				server.StartTLS()
				defer server.Close()
				roots := x509.NewCertPool()
				roots.AddCert(server.Certificate())
				client, err := newLauncherHTTPClient(roots)
				if err != nil {
					t.Fatal(err)
				}
				defer client.CloseIdleConnections()
				a, err := newAuthenticator(client)
				if err != nil {
					t.Fatal(err)
				}
				var observation HTTPObservation
				a.Observe = func(o HTTPObservation) { observation = o }
				data, err := a.request(context.Background(), "login", server.URL+"/Login.ashx?p=private-p&key=private-key", "")
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(data, gameBytes) {
					t.Fatal("HTTP decoding changed XOR game bytes")
				}
				decoded, err := decodeLoginResponse(data)
				if err != nil || !bytes.Equal(decoded, plain) {
					t.Fatalf("game response decoding failed: %v", err)
				}
				wantProtocol := "HTTP/1.1"
				if h2 {
					wantProtocol = "HTTP/2.0"
				}
				if observation.Protocol != wantProtocol || observation.Kind != "" || observation.StatusCode != 200 || observation.RayID != "fixture-ray-GRU" || observation.Stage != "login" || observation.Path != "/Login.ashx" || observation.Duration <= 0 {
					t.Fatalf("observation = %#v", observation)
				}
				if strings.Contains(fmt.Sprint(observation), "private-") {
					t.Fatal("observation leaked query credentials")
				}
				if !classicHello.Load() {
					t.Fatal("configured legacy client hello was not emitted")
				}
			})
		}
	}
}

func TestLauncherTransportRefusesRedirectAndUntrustedCertificate(t *testing.T) {
	var followed atomic.Bool
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/target" {
			followed.Store(true)
			return
		}
		w.Header().Set("Location", "/target")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()
	for _, trusted := range []bool{true, false} {
		t.Run(fmt.Sprintf("trusted=%v", trusted), func(t *testing.T) {
			var roots *x509.CertPool
			if trusted {
				roots = x509.NewCertPool()
				roots.AddCert(server.Certificate())
			}
			client, err := newLauncherHTTPClient(roots)
			if err != nil {
				t.Fatal(err)
			}
			defer client.CloseIdleConnections()
			req, _ := http.NewRequest(http.MethodGet, server.URL+"/redirect", nil)
			response, err := client.Do(req)
			if !trusted {
				if err == nil {
					response.Body.Close()
					t.Fatal("self-signed certificate accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != 302 || followed.Load() {
				t.Fatal("authentication redirect was followed")
			}
		})
	}
}

func TestLauncherTransportHonorsCancellation(t *testing.T) {
	client, err := newLauncherHTTPClient(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://127.0.0.1:1/Login.ashx?p=private", nil)
	_, err = client.Do(req)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestLauncherDeflateHandlesRawAndMalformedStreams(t *testing.T) {
	plain := []byte(`<Result value="true"><Item ID="123" /></Result>`)
	for _, tc := range []struct {
		name  string
		wire  []byte
		valid bool
	}{
		{"raw", encodeHTTP(t, plain, "raw-deflate"), true},
		{"malformed-zlib", []byte{0x78, 0x9c, 0xff, 0xff, 0x00}, false},
		{"empty", nil, false},
		{"one-byte", []byte{0x78}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Errorf("deflate decoder panicked: %v", recovered)
				}
			}()
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Encoding", "deflate")
				w.Write(tc.wire)
			}))
			defer server.Close()
			roots := x509.NewCertPool()
			roots.AddCert(server.Certificate())
			client, err := newLauncherHTTPClient(roots)
			if err != nil {
				t.Fatal(err)
			}
			defer client.CloseIdleConnections()
			a, err := newAuthenticator(client)
			if err != nil {
				t.Fatal(err)
			}
			body, err := a.request(context.Background(), "login", server.URL+"/Login.ashx", "")
			if tc.valid && (err != nil || !bytes.Equal(body, plain)) {
				t.Fatalf("raw deflate bytes changed: error=%v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("malformed deflate response accepted")
			}
		})
	}
}

func TestMalformedCompressed403StillTriggersForbiddenClassification(t *testing.T) {
	for _, encoding := range []string{"gzip", "deflate"} {
		t.Run(encoding, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Encoding", encoding)
				w.Header().Set("CF-Ray", "fixture-ray-GRU")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte{0x78, 0xbb})
			}))
			defer server.Close()
			roots := x509.NewCertPool()
			roots.AddCert(server.Certificate())
			client, err := newLauncherHTTPClient(roots)
			if err != nil {
				t.Fatal(err)
			}
			defer client.CloseIdleConnections()
			a, err := newAuthenticator(client)
			if err != nil {
				t.Fatal(err)
			}
			_, err = a.request(context.Background(), "login", server.URL+"/Login.ashx", "")
			var failure *Error
			if !errors.As(err, &failure) || failure.Kind != "forbidden" || failure.StatusCode != 403 || failure.RayID != "fixture-ray-GRU" {
				t.Fatalf("HTTP forbidden response lost behind decoding failure: %v", err)
			}
		})
	}
}

func TestLauncherTransportKeepsCookieJarWithinOneClient(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/set" {
			http.SetCookie(w, &http.Cookie{Name: "session-fixture", Value: "private-fixture", Path: "/", Secure: true})
			return
		}
		if _, err := r.Cookie("session-fixture"); err == nil {
			_, _ = io.WriteString(w, "present")
		} else {
			_, _ = io.WriteString(w, "absent")
		}
	}))
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	one, err := newLauncherHTTPClient(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer one.CloseIdleConnections()
	two, err := newLauncherHTTPClient(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer two.CloseIdleConnections()
	for _, step := range []struct {
		client     *launcherHTTPClient
		path, want string
	}{{one, "/set", ""}, {one, "/get", "present"}, {two, "/get", "absent"}} {
		req, _ := http.NewRequest(http.MethodGet, server.URL+step.path, nil)
		response, err := step.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || string(body) != step.want {
			t.Fatalf("cookie isolation result = %q, error=%v", body, err)
		}
	}
}

func TestLauncherLoadedAuthAndOtherCookiesShareTheWireWithoutDuplicates(t *testing.T) {
	private, err := rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"ddclassico.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true, IsCA: true}
	certificate, err := x509.CreateCertificate(cryptorand.Reader, template, template, &private.PublicKey, private)
	if err != nil {
		t.Fatal(err)
	}
	for _, h2 := range []bool{false, true} {
		t.Run(fmt.Sprintf("h2=%v", h2), func(t *testing.T) {
			current := "fresh-file-cookie-one"
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count := 0
				for _, cookie := range r.Cookies() {
					if cookie.Name == ".Auth" {
						count++
						if cookie.Value != current {
							t.Error("launcher sent stale .Auth")
						}
					}
				}
				if count != 1 {
					t.Errorf("launcher sent %d .Auth cookies", count)
				}
				if cookie, err := r.Cookie("cf_clearance"); err != nil || cookie.Value != "clearance-fixture" {
					t.Error("launcher lost native jar cookie")
				}
				http.SetCookie(w, &http.Cookie{Name: ".Auth", Value: "server-renewed-cookie", Path: "/", Secure: true})
				_, _ = io.WriteString(w, "ok")
			}))
			server.EnableHTTP2 = h2
			server.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{certificate}, PrivateKey: private}}}
			server.StartTLS()
			defer server.Close()
			roots := x509.NewCertPool()
			roots.AddCert(server.Certificate())
			client, err := newLauncherHTTPClient(roots, tlsclient.WithDialContext(func(ctx context.Context, network, address string) (net.Conn, error) {
				if address != "ddclassico.com:443" {
					return nil, errors.New("unexpected test endpoint")
				}
				return (&net.Dialer{}).DialContext(ctx, "tcp", server.Listener.Addr().String())
			}))
			if err != nil {
				t.Fatal(err)
			}
			defer client.CloseIdleConnections()
			origin, _ := url.Parse(websiteURL)
			client.client.SetCookies(origin, []*fhttp.Cookie{{Name: ".Auth", Value: "stale-jar-cookie", Path: "/", Secure: true}, {Name: "cf_clearance", Value: "clearance-fixture", Path: "/", Secure: true}})
			a, err := newAuthenticator(client)
			if err != nil {
				t.Fatal(err)
			}
			for _, cookie := range []string{"fresh-file-cookie-one", "fresh-file-cookie-two"} {
				current = cookie
				if _, err := a.request(context.Background(), "website", websiteURL+"/classic/playgame/456", cookie); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestLauncherExpandedHTTPResponseHonorsLimit(t *testing.T) {
	for _, encoding := range []string{"gzip", "br", "deflate"} {
		t.Run(encoding, func(t *testing.T) {
			wire := encodeHTTP(t, bytes.Repeat([]byte("X"), maxHTTPBytes+1), encoding)
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Encoding", encoding)
				w.Write(wire)
			}))
			defer server.Close()
			roots := x509.NewCertPool()
			roots.AddCert(server.Certificate())
			client, err := newLauncherHTTPClient(roots)
			if err != nil {
				t.Fatal(err)
			}
			defer client.CloseIdleConnections()
			a, err := newAuthenticator(client)
			if err != nil {
				t.Fatal(err)
			}
			_, err = a.request(context.Background(), "login", server.URL+"/Login.ashx", "")
			var failure *Error
			if !errors.As(err, &failure) || failure.Kind != "protocol" || !strings.Contains(failure.Reason, "exceeds limit") {
				t.Fatalf("expanded HTTP entity limit failed: %v", err)
			}
		})
	}
}

func encodeHTTP(t *testing.T, data []byte, encoding string) []byte {
	t.Helper()
	if encoding == "identity" {
		return bytes.Clone(data)
	}
	var buf bytes.Buffer
	var writer io.WriteCloser
	switch encoding {
	case "gzip":
		writer = gzip.NewWriter(&buf)
	case "br":
		writer = brotli.NewWriter(&buf)
	case "deflate":
		writer = zlib.NewWriter(&buf)
	case "raw-deflate":
		var err error
		writer, err = flate.NewWriter(&buf, flate.DefaultCompression)
		if err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("unsupported fixture encoding")
	}
	if _, err := writer.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
