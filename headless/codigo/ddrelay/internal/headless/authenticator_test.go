package headless

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
	"time"
)

type authRoundTrip func(*http.Request) (*http.Response, error)

func (f authRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func authResponse(code int, body []byte) *http.Response {
	return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body))}
}

func testAuthenticator(t *testing.T, login func(*http.Request) (*http.Response, error)) *Authenticator {
	t.Helper()
	a, err := NewAuthenticator(&http.Client{Transport: authRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "ddclassico.com" {
			if r.URL.Path != "/classic/playgame/456" {
				t.Errorf("website path = %q", r.URL.Path)
			}
			cookie, err := r.Cookie(".Auth")
			if err != nil || cookie.Value != "private-cookie-fixture" {
				t.Error("website request did not carry account cookie")
			}
			return authResponse(200, []byte(`<embed src="https://resource.ddclassico.com/flash/Loading.swf?user=fixture-user&amp;key=fixture-key">`)), nil
		}
		if r.URL.Host != "s1.ddclassico.com" || r.URL.Path != "/Login.ashx" {
			return nil, errors.New("unexpected endpoint")
		}
		if r.Header.Get("Cookie") != "" {
			t.Error("website cookie leaked into game login request")
		}
		return login(r)
	})})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

var fixtureCharacter = Character{ID: 123, WebsiteID: 456, Name: "Fixture", Upstream: "127.0.0.1:1401"}

func TestBlockedLoginExplainsStageAndRayWithoutCredentials(t *testing.T) {
	a := testAuthenticator(t, func(*http.Request) (*http.Response, error) {
		r := authResponse(403, []byte(`<html><title>Attention Required! | Cloudflare</title>Sorry, you have been blocked</html>`))
		r.Header.Set("Server", "cloudflare")
		r.Header.Set("CF-Ray", "a59faf1cc91ea448-GRU")
		return r, nil
	})
	_, err := a.Authenticate(context.Background(), AccountSession{Cookie: "private-cookie-fixture"}, fixtureCharacter)
	if err == nil {
		t.Fatal("blocked login returned a ticket")
	}
	for _, want := range []string{"blocked", "login", "s1.ddclassico.com", "a59faf1cc91ea448-GRU"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("diagnostic %q does not contain %q", err, want)
		}
	}
	for _, secret := range []string{"private-cookie-fixture", "fixture-user", "fixture-key", "?", "<html>"} {
		if strings.Contains(err.Error(), secret) {
			t.Errorf("diagnostic exposes %q", secret)
		}
	}
}

func TestNewAuthenticatorRejectsNilClient(t *testing.T) {
	if _, err := NewAuthenticator(nil); err == nil {
		t.Fatal("nil HTTP client accepted")
	}
}

func TestConfiguredLauncherTransportIsAvailable(t *testing.T) {
	if _, err := NewConfiguredAuthenticator(Config{Auth: AuthConfig{Transport: "launcher_chromium87"}}); err != nil {
		t.Fatalf("configured transport is unavailable: %v", err)
	}
}

func TestHTTPFailuresAreTypedBeforeGameDecoding(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		header     http.Header
		body, kind string
	}{
		{"challenge header", 403, http.Header{"Cf-Mitigated": {"challenge"}}, "opaque", "challenge"},
		{"challenge with 200", 200, nil, `<html><title>Just a moment...</title><script src="/cdn-cgi/challenge-platform/">Cloudflare</script></html>`, "challenge"},
		{"unknown 403 behind cloudflare", 403, http.Header{"Server": {"cloudflare"}}, "denied by application", "forbidden"},
		{"rate limit", 429, http.Header{"Retry-After": {"120"}}, "slow down", "network"},
		{"unavailable", 503, nil, "upstream unavailable", "network"},
		{"rejected character", 200, nil, `<Result value="false" />`, "rejected"},
		{"wrong character", 200, nil, `<Result value="true"><Item ID="999" /></Result>`, "config"},
		{"multiple characters", 200, nil, `<Result value="true"><Item ID="123" /><Item ID="124" /></Result>`, "rejected"},
		{"invalid response", 200, nil, "bad compressed response", "protocol"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := testAuthenticator(t, func(*http.Request) (*http.Response, error) {
				r := authResponse(tc.status, []byte(tc.body))
				for k, v := range tc.header {
					r.Header[k] = v
				}
				return r, nil
			})
			_, err := a.Authenticate(context.Background(), AccountSession{Cookie: "private-cookie-fixture"}, fixtureCharacter)
			var failure *Error
			if !errors.As(err, &failure) || failure.Kind != tc.kind || failure.Stage != "login" {
				t.Fatalf("failure = %#v", err)
			}
			if tc.status == 429 && failure.RetryAfter != 120*time.Second {
				t.Fatalf("retry-after = %v", failure.RetryAfter)
			}
		})
	}
}

func TestLoadedCookieOverridesStaleJarAuthWithoutDroppingOtherCookies(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	origin, _ := url.Parse(websiteURL)
	jar.SetCookies(origin, []*http.Cookie{{Name: ".Auth", Value: "stale-jar-cookie", Path: "/"}, {Name: "cf_clearance", Value: "clearance-fixture", Path: "/"}})
	current := "fresh-file-cookie-one"
	client := &http.Client{Jar: jar, Transport: authRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "ddclassico.com" {
			count := 0
			for _, cookie := range r.Cookies() {
				if cookie.Name == ".Auth" {
					count++
					if cookie.Value != current {
						t.Error("stale jar cookie overrode session file")
					}
				}
			}
			if count != 1 {
				t.Errorf("website received %d .Auth cookies", count)
			}
			if cookie, err := r.Cookie("cf_clearance"); err != nil || cookie.Value != "clearance-fixture" {
				t.Error("other scoped cookie lost")
			}
			response := authResponse(200, []byte(`<embed src="https://resource.ddclassico.com/flash/Loading.swf?user=fixture-user&amp;key=fixture-key">`))
			response.Header.Add("Set-Cookie", ".Auth=server-renewed-cookie; Path=/; Secure")
			return response, nil
		}
		return authResponse(200, []byte(`<Result value="true"><Item ID="123" /></Result>`)), nil
	})}
	a, err := NewAuthenticator(client)
	if err != nil {
		t.Fatal(err)
	}
	for _, cookie := range []string{"fresh-file-cookie-one", "fresh-file-cookie-two"} {
		current = cookie
		if _, err := a.Authenticate(context.Background(), AccountSession{Cookie: cookie}, fixtureCharacter); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTransportErrorsKeepCausePrivate(t *testing.T) {
	cause := errors.New("GET https://s1.ddclassico.com/Login.ashx?p=private-p&key=private-key")
	a := testAuthenticator(t, func(*http.Request) (*http.Response, error) { return nil, cause })
	_, err := a.Authenticate(context.Background(), AccountSession{Cookie: "private-cookie-fixture"}, fixtureCharacter)
	if !errors.Is(err, cause) {
		t.Fatal("transport cause lost")
	}
	if strings.Contains(err.Error(), "private-") || strings.Contains(err.Error(), "?") {
		t.Fatalf("transport diagnostic leaked URL: %v", err)
	}
}

func TestAdapterProtocolFailureRetainsSafeMetadata(t *testing.T) {
	a := testAuthenticator(t, func(*http.Request) (*http.Response, error) {
		return nil, &Error{Kind: "protocol", Reason: "unsupported HTTP content encoding", StatusCode: 200, RayID: "fixture-ray-GRU"}
	})
	_, err := a.Authenticate(context.Background(), AccountSession{Cookie: "private-cookie-fixture"}, fixtureCharacter)
	var failure *Error
	if !errors.As(err, &failure) || failure.Kind != "protocol" || failure.Stage != "login" || failure.StatusCode != 200 || failure.RayID != "fixture-ray-GRU" {
		t.Fatalf("typed adapter failure lost: %v", err)
	}
}

func TestErrorJSONExcludesSensitiveCause(t *testing.T) {
	failure := &Error{Kind: "network", Reason: "transport failed", Cause: &url.Error{Op: "Get", URL: "https://example.invalid/?cookie=private-cookie", Err: errors.New("private-password")}}
	data, err := json.Marshal(failure)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "private-") || strings.Contains(string(data), "example.invalid") {
		t.Fatalf("JSON error leaked cause: %s", data)
	}
}

func TestHTTPAndDecodedLoginLimits(t *testing.T) {
	for _, compressed := range []bool{false, true} {
		t.Run(fmt.Sprintf("compressed=%v", compressed), func(t *testing.T) {
			body := bytes.Repeat([]byte("X"), maxHTTPBytes+1)
			if compressed {
				body = obfuscatedLogin(t, body)
			}
			a := testAuthenticator(t, func(*http.Request) (*http.Response, error) { return authResponse(200, body), nil })
			_, err := a.Authenticate(context.Background(), AccountSession{Cookie: "private-cookie-fixture"}, fixtureCharacter)
			var failure *Error
			if !errors.As(err, &failure) || failure.Kind != "protocol" || !strings.Contains(failure.Reason, "exceeds limit") {
				t.Fatalf("oversized response accepted: %v", err)
			}
		})
	}
}

func TestLoginPreservesEncryptedCredentialsAndBinaryResponse(t *testing.T) {
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	for _, compressed := range []bool{false, true} {
		t.Run(fmt.Sprintf("compressed=%v", compressed), func(t *testing.T) {
			var password string
			a := testAuthenticator(t, func(r *http.Request) (*http.Response, error) {
				ciphertext, err := base64.StdEncoding.DecodeString(r.URL.Query().Get("p"))
				if err != nil {
					t.Fatal(err)
				}
				plain, err := rsa.DecryptPKCS1v15(rand.Reader, private, ciphertext)
				if err != nil {
					t.Fatal(err)
				}
				if len(plain) < 7 {
					t.Fatal("missing timestamp")
				}
				fields := strings.Split(string(plain[7:]), ",")
				if len(fields) != 4 || fields[0] != "fixture-user" || fields[1] != "fixture-key" || fields[3] != "Fixture" {
					t.Fatal("encrypted login fields changed")
				}
				password = fields[2]
				if len(password) != 6 || strings.Trim(password, "abcdefghijklmnopqrstuvwxyz") != "" {
					t.Fatal("invalid temporary password")
				}
				if r.URL.Query().Get("v") != "5498628" || r.URL.Query().Get("ip") != "null" || r.URL.Query().Get("selfid") != "NaN" {
					t.Fatal("login parameters changed")
				}
				if r.URL.Query().Get("key") != "f9c639ae9781ea930eafcc42425c0e9d" {
					t.Fatal("session key digest changed")
				}
				body := []byte(`<Result value="true"><Item ID="123" /></Result>`)
				if compressed {
					body = obfuscatedLogin(t, body)
				}
				return authResponse(200, body), nil
			})
			a.key = &private.PublicKey
			ticket, err := a.Authenticate(context.Background(), AccountSession{Cookie: "private-cookie-fixture"}, fixtureCharacter)
			if err != nil {
				t.Fatal(err)
			}
			if ticket.Character != fixtureCharacter || ticket.User != "fixture-user" || ticket.Password != password {
				t.Fatal("returned ticket differs from validated credentials")
			}
		})
	}
}

func obfuscatedLogin(t *testing.T, plain []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(plain); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	key := []byte{127, 95, 200, 255, 15, 34, 78, 90}
	for i := 0; i < len(data) && i < 1024; i++ {
		data[i] ^= key[i%8]
	}
	return data
}
