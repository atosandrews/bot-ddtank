package headless

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"html"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	clientBuild   = 5498628
	maxHTTPBytes  = 4 << 20
	websiteURL    = "https://ddclassico.com"
	loginURL      = "https://s1.ddclassico.com/Login.ashx"
	launcherAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) DDClassico/1.0.5 Chrome/87.0.4280.141 Electron/11.5.0 Safari/537.36 DDTankClassico/0.1.0.0 DDClassico/0.1.0.0"
	publicModulus = "9etZSHNUPAbmwVjW3Rlnf/vaK+u1WLFzMmdzGytWWWdvmjostHZKnE8xpU42qDnSLvDNRkGwJJ6ceiijZ0GLy2YhtJ0W8BgT7SGNHmTE5RCOoR/sbDeuNk/19Tv9aPodKDysccsyMJB3uhODN6PYUjUe7/e8J/hxPFuc4Q3W9ek8cJWAv++8mDSZ06+BMUljTERnsoWygWfWe2je0sBmxKh6xwhWXvvAmNY0fEJ2S1YnnXWc8zm9gBbE58yQWeaVc8qcHv0wsdyK4Ge9h/lGC4jtNPLhqQgWPweV3pQA4OfD68jMOPcTs/RioBG/xqsoBTjEyzn85o4yWoVetSwWSQ=="
)

type Authenticator struct {
	client AuthHTTP
	key    *rsa.PublicKey
	launch *regexp.Regexp
	// Observe must be configured before Authenticate is called. Concurrent calls
	// invoke it concurrently; observations never include credentials or query data.
	Observe func(HTTPObservation)
}

type HTTPObservation struct {
	Stage, Host, Path, Protocol, RayID, Kind string
	StatusCode                               int
	Duration                                 time.Duration
}

func NewAuthenticator(client *http.Client) (*Authenticator, error) {
	if client == nil {
		return nil, &Error{Operation: "initialize", Kind: "config", Reason: "HTTP client required"}
	}
	clone := *client
	if clone.Jar != nil {
		clone.Jar = websiteSessionJar{CookieJar: clone.Jar}
	}
	return newAuthenticator(&clone)
}

func newAuthenticator(client AuthHTTP) (*Authenticator, error) {
	if client == nil {
		return nil, &Error{Operation: "initialize", Kind: "config", Reason: "HTTP client required"}
	}
	modulus, err := base64.StdEncoding.DecodeString(publicModulus)
	if err != nil {
		return nil, &Error{Operation: "initialize", Kind: "config", Reason: "invalid public key", Cause: err}
	}

	return &Authenticator{
		client: client,
		key:    &rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: 65537},
		launch: regexp.MustCompile(`https://resource\.ddclassico\.com/flash/Loading\.swf\?[^"<>\s]+`),
	}, nil
}

func (a *Authenticator) Authenticate(ctx context.Context, session AccountSession, character Character) (Ticket, error) {
	page := websiteURL + "/classic/playgame/" + strconv.FormatInt(int64(character.WebsiteID), 10)
	body, err := a.request(ctx, "website", page, session.Cookie)
	if err != nil {
		return Ticket{}, err
	}

	launch := a.launch.Find(body)
	if len(launch) == 0 {
		return Ticket{}, &Error{Operation: "launch", Kind: "session", Stage: "website", Host: "ddclassico.com", Path: "/classic/playgame/" + strconv.FormatInt(int64(character.WebsiteID), 10), Reason: "no character ticket; account session may have expired"}
	}

	parsed, err := url.Parse(html.UnescapeString(string(launch)))
	if err != nil {
		return Ticket{}, &Error{Operation: "launch", Kind: "protocol", Stage: "website", Reason: "invalid character ticket", Cause: err}
	}

	query := parsed.Query()
	user, key := query.Get("user"), query.Get("key")
	if user == "" || key == "" || strings.ContainsAny(user+key+character.Name, ",\x00\r\n") {
		return Ticket{}, &Error{Operation: "launch", Kind: "protocol", Stage: "website", Reason: "invalid credential fields"}
	}

	password, err := randomPassword()
	if err != nil {
		return Ticket{}, err
	}

	plain := append(timestamp(time.Now().UTC()), []byte(user+","+key+","+password+","+character.Name)...)
	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, a.key, plain)
	if err != nil {
		return Ticket{}, &Error{Operation: "login", Kind: "protocol", Stage: "login", Reason: "cannot encode login", Cause: err}
	}

	digest := md5.Sum([]byte(key))
	parameters := url.Values{
		"ip": {"null"}, "selfid": {"NaN"}, "v": {strconv.Itoa(clientBuild)},
		"key": {hex.EncodeToString(digest[:])}, "p": {base64.StdEncoding.EncodeToString(encrypted)},
	}
	response, err := a.request(ctx, "login", loginURL+"?"+parameters.Encode(), "")
	if err != nil {
		return Ticket{}, err
	}

	decoded, err := decodeLoginResponse(response)
	if err != nil {
		return Ticket{}, err
	}

	var result loginResult
	if err := xml.Unmarshal(decoded, &result); err != nil {
		return Ticket{}, &Error{Operation: "login", Kind: "protocol", Stage: "login", Host: "s1.ddclassico.com", Path: "/Login.ashx", Reason: "invalid login response", Cause: err}
	}

	if !result.Accepted || len(result.Items) != 1 {
		return Ticket{}, &Error{Operation: "login", Kind: "rejected", Stage: "login", Reason: "server rejected login"}
	}

	if result.Items[0].ID != character.ID {
		return Ticket{}, &Error{Operation: "login", Kind: "config", Stage: "login", Reason: "server returned character " + strconv.FormatInt(int64(result.Items[0].ID), 10) + ", configured " + strconv.FormatInt(int64(character.ID), 10)}
	}

	return Ticket{Character: character, User: user, Password: password}, nil
}

func (a *Authenticator) request(ctx context.Context, stage, address, cookie string) ([]byte, error) {
	started := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, &Error{Operation: "request", Kind: "protocol", Stage: stage, Reason: "cannot construct request", Cause: err}
	}
	diagnostic := Error{Operation: "request", Kind: "network", Stage: stage, Host: req.URL.Hostname(), Path: req.URL.EscapedPath()}
	protocol := ""
	observe := a.Observe
	defer func() {
		if observe != nil {
			observe(HTTPObservation{Stage: stage, Host: diagnostic.Host, Path: diagnostic.Path, Protocol: protocol, RayID: diagnostic.RayID, Kind: diagnostic.Kind, StatusCode: diagnostic.StatusCode, Duration: time.Since(started)})
		}
	}()

	req.Header.Set("User-Agent", launcherAgent)
	req.Header.Set("Accept", "*/*")
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: ".Auth", Value: cookie})
	}

	response, err := a.client.Do(req)
	if err != nil {
		diagnostic.Reason, diagnostic.Cause = "HTTP transport failed", err
		var transportError *Error
		if errors.As(err, &transportError) && transportError.Kind != "" {
			diagnostic.Kind, diagnostic.Reason = transportError.Kind, transportError.Reason
			diagnostic.StatusCode, diagnostic.RayID = transportError.StatusCode, safeRayID(transportError.RayID)
			diagnostic.RetryAfter = transportError.RetryAfter
		}
		return nil, &diagnostic
	}
	diagnostic.StatusCode = response.StatusCode
	protocol = response.Proto
	diagnostic.RayID = safeRayID(response.Header.Get("CF-Ray"))
	diagnostic.RetryAfter = parseRetryAfter(response.Header.Get("Retry-After"), time.Now())

	data, readErr := io.ReadAll(io.LimitReader(response.Body, maxHTTPBytes+1))
	closeErr := response.Body.Close()
	if kind := classifyHTTP(stage, response.StatusCode, response.Header, data); kind != "" {
		diagnostic.Kind, diagnostic.Reason = kind, "HTTP response rejected"
		diagnostic.Cause = errors.Join(readErr, closeErr)
		return nil, &diagnostic
	}
	if err := errors.Join(readErr, closeErr); err != nil {
		diagnostic.Reason, diagnostic.Cause = "cannot read HTTP response", err
		var decodingError *Error
		if errors.As(err, &decodingError) && decodingError.Kind == "protocol" {
			diagnostic.Kind = "protocol"
		}
		return nil, &diagnostic
	}

	if len(data) > maxHTTPBytes {
		diagnostic.Kind, diagnostic.Reason = "protocol", "HTTP response exceeds limit"
		return nil, &diagnostic
	}

	diagnostic.Kind = ""
	return data, nil
}

// CloseIdleConnections releases pooled HTTP connections during application shutdown.
func (a *Authenticator) CloseIdleConnections() {
	if client, ok := a.client.(interface{ CloseIdleConnections() }); ok {
		client.CloseIdleConnections()
	}
}

func decodeLoginResponse(data []byte) ([]byte, error) {
	if bytes.HasPrefix(bytes.TrimSpace(data), []byte("<")) {
		return data, nil
	}

	key := [...]byte{127, 95, 200, 255, 15, 34, 78, 90}
	decoded := bytes.Clone(data)
	for i := 0; i < min(1024, len(decoded)); i++ {
		decoded[i] ^= key[i%len(key)]
	}

	reader, err := zlib.NewReader(bytes.NewReader(decoded))
	if err != nil {
		return nil, &Error{Operation: "login", Kind: "protocol", Stage: "login", Host: "s1.ddclassico.com", Path: "/Login.ashx", Reason: "invalid compressed response", Cause: err}
	}

	result, readErr := io.ReadAll(io.LimitReader(reader, maxHTTPBytes+1))
	closeErr := reader.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, &Error{Operation: "login", Kind: "protocol", Stage: "login", Reason: "cannot decompress response", Cause: err}
	}

	if len(result) > maxHTTPBytes {
		return nil, &Error{Operation: "login", Kind: "protocol", Stage: "login", Reason: "decompressed response exceeds limit"}
	}

	return result, nil
}

func randomPassword() (string, error) {
	password := make([]byte, 6)
	for i := range password {
		value, err := rand.Int(rand.Reader, big.NewInt(26))
		if err != nil {
			return "", &Error{Operation: "login", Kind: "protocol", Stage: "login", Reason: "random generation failed", Cause: err}
		}

		password[i] = byte(value.Int64()) + 'a'
	}

	return string(password), nil
}
