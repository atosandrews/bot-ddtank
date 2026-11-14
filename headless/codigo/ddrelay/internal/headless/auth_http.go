package headless

import (
	"net/http"
	"net/http/cookiejar"
	"time"
)

// AuthHTTP returns HTTP entity bytes after Content-Encoding has been decoded.
// The game-specific XOR/zlib decoding belongs to Authenticator, not the client.
type AuthHTTP interface {
	Do(*http.Request) (*http.Response, error)
}

func NewConfiguredAuthenticator(config Config) (*Authenticator, error) {
	if config.Auth.Transport == "launcher_chromium87" {
		client, err := newLauncherHTTPClient(nil)
		if err != nil {
			return nil, err
		}
		return newAuthenticator(client)
	}
	if config.Auth.Transport != "" && config.Auth.Transport != "standard" {
		return nil, &Error{Operation: "initialize", Kind: "config", Reason: "unsupported authentication transport"}
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, &Error{Operation: "initialize", Kind: "config", Reason: "cannot initialize cookie jar", Cause: err}
	}
	return NewAuthenticator(&http.Client{
		Timeout: 20 * time.Second, Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	})
}
