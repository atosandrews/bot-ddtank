package headless

import (
	"crypto/x509"
	"net/http"
	"strings"

	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/fhttp/http2"
	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	utls "github.com/bogdanfinn/utls"
)

type launcherHTTPClient struct{ client tlsclient.HttpClient }

// launcherChromium87 is a fixed compatibility candidate, not a fingerprint
// measured from an accepted launcher session. Keep TLS and HTTP/2 settings
// together and promote it only after the real HTTP and game login gates pass.
func launcherChromium87() profiles.ClientProfile {
	return profiles.NewClientProfile(utls.HelloChrome_87,
		map[http2.SettingID]uint32{
			http2.SettingHeaderTableSize:      65536,
			http2.SettingMaxConcurrentStreams: 1000,
			http2.SettingInitialWindowSize:    6291456,
			http2.SettingMaxHeaderListSize:    262144,
		},
		[]http2.SettingID{http2.SettingHeaderTableSize, http2.SettingMaxConcurrentStreams, http2.SettingInitialWindowSize, http2.SettingMaxHeaderListSize},
		[]string{":method", ":authority", ":scheme", ":path"}, 15663105, nil,
		&http2.PriorityParam{StreamDep: 0, Exclusive: true, Weight: 255}, 0, false, nil, nil, 0, nil, false,
	)
}

func newLauncherHTTPClient(roots *x509.CertPool, extra ...tlsclient.HttpClientOption) (*launcherHTTPClient, error) {
	options := []tlsclient.HttpClientOption{
		tlsclient.WithClientProfile(launcherChromium87()),
		tlsclient.WithTimeoutSeconds(20),
		tlsclient.WithNotFollowRedirects(),
		tlsclient.WithDisableHttp3(),
		tlsclient.WithCookieJar(launcherSessionJar{CookieJar: tlsclient.NewCookieJar()}),
		// Own HTTP decompression here for identical behavior on HTTP/1 and HTTP/2.
		// Random extension order and protocol racing are deliberately never enabled.
		tlsclient.WithTransportOptions(&tlsclient.TransportOptions{RootCAs: roots, DisableCompression: true}),
	}
	client, err := tlsclient.NewHttpClient(tlsclient.NewNoopLogger(), append(options, extra...)...)
	if err != nil {
		return nil, &Error{Operation: "initialize", Kind: "config", Reason: "cannot initialize launcher transport", Cause: err}
	}
	return &launcherHTTPClient{client: client}, nil
}

func (c *launcherHTTPClient) Do(req *http.Request) (*http.Response, error) {
	r, err := fhttp.NewRequestWithContext(req.Context(), req.Method, req.URL.String(), req.Body)
	if err != nil {
		return nil, err
	}
	r.Host = req.Host
	for name, values := range req.Header {
		r.Header[http.CanonicalHeaderKey(name)] = append([]string(nil), values...)
	}
	if r.Header.Get("Accept-Encoding") == "" {
		r.Header.Set("Accept-Encoding", "gzip, deflate, br")
	}
	r.Header[fhttp.HeaderOrderKey] = []string{"user-agent", "accept", "accept-encoding", "cookie"}
	response, err := c.client.Do(r)
	if err != nil {
		return nil, err
	}
	header := make(http.Header, len(response.Header))
	for name, values := range response.Header {
		header[http.CanonicalHeaderKey(name)] = append([]string(nil), values...)
	}
	encoding := strings.ToLower(strings.TrimSpace(header.Get("Content-Encoding")))
	body := response.Body
	if encoding != "" && encoding != "identity" {
		body, err = decodeHTTPBody(body, encoding)
		// Preserve status and Cloudflare headers even when an error page has a
		// malformed compressed body. Authenticator classifies a 403/challenge
		// before reporting a body error, so it can still open the shared gate.
		if err != nil {
			body = failedHTTPBody{err: err}
		}
		header.Del("Content-Encoding")
		header.Del("Content-Length")
	}
	return &http.Response{StatusCode: response.StatusCode, Status: response.Status, Proto: response.Proto, ProtoMajor: response.ProtoMajor, ProtoMinor: response.ProtoMinor, Header: header, Body: body, Request: req, ContentLength: -1, Uncompressed: encoding != "" && encoding != "identity"}, nil
}

func (c *launcherHTTPClient) CloseIdleConnections() { c.client.CloseIdleConnections() }
