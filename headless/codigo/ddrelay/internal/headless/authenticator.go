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
	client *http.Client
	key    *rsa.PublicKey
	launch *regexp.Regexp
}

func NewAuthenticator(client *http.Client) (*Authenticator, error) {
	modulus, err := base64.StdEncoding.DecodeString(publicModulus)
	if err != nil {
		return nil, &Error{Operation: "initialize", Reason: "invalid public key", Cause: err}
	}

	return &Authenticator{
		client: client,
		key:    &rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: 65537},
		launch: regexp.MustCompile(`https://resource\.ddclassico\.com/flash/Loading\.swf\?[^"<>\s]+`),
	}, nil
}

func (a *Authenticator) Authenticate(ctx context.Context, session AccountSession, character Character) (Ticket, error) {
	page := websiteURL + "/classic/playgame/" + strconv.FormatInt(int64(character.WebsiteID), 10)
	body, err := a.request(ctx, page, session.Cookie)
	if err != nil {
		return Ticket{}, err
	}

	launch := a.launch.Find(body)
	if len(launch) == 0 {
		return Ticket{}, &Error{Operation: "launch", Reason: "no character ticket; account session may have expired"}
	}

	parsed, err := url.Parse(html.UnescapeString(string(launch)))
	if err != nil {
		return Ticket{}, &Error{Operation: "launch", Reason: "invalid character ticket", Cause: err}
	}

	query := parsed.Query()
	user, key := query.Get("user"), query.Get("key")
	if user == "" || key == "" || strings.ContainsAny(user+key+character.Name, ",\x00\r\n") {
		return Ticket{}, &Error{Operation: "launch", Reason: "invalid credential fields"}
	}

	password, err := randomPassword()
	if err != nil {
		return Ticket{}, err
	}

	plain := append(timestamp(time.Now().UTC()), []byte(user+","+key+","+password+","+character.Name)...)
	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, a.key, plain)
	if err != nil {
		return Ticket{}, &Error{Operation: "login", Reason: "cannot encode login", Cause: err}
	}

	digest := md5.Sum([]byte(key))
	parameters := url.Values{
		"ip": {"null"}, "selfid": {"NaN"}, "v": {strconv.Itoa(clientBuild)},
		"key": {hex.EncodeToString(digest[:])}, "p": {base64.StdEncoding.EncodeToString(encrypted)},
	}
	response, err := a.request(ctx, loginURL+"?"+parameters.Encode(), "")
	if err != nil {
		return Ticket{}, err
	}

	decoded, err := decodeLoginResponse(response)
	if err != nil {
		return Ticket{}, err
	}

	var result loginResult
	if err := xml.Unmarshal(decoded, &result); err != nil {
		return Ticket{}, &Error{Operation: "login", Reason: "invalid login response", Cause: err}
	}

	if !result.Accepted || len(result.Items) != 1 {
		return Ticket{}, &Error{Operation: "login", Reason: "server rejected login"}
	}

	if result.Items[0].ID != character.ID {
		return Ticket{}, &Error{Operation: "login", Reason: "server returned character " + strconv.FormatInt(int64(result.Items[0].ID), 10) + ", configured " + strconv.FormatInt(int64(character.ID), 10)}
	}

	return Ticket{Character: character, User: user, Password: password}, nil
}

func (a *Authenticator) request(ctx context.Context, address, cookie string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, &Error{Operation: "request", Reason: "cannot construct request", Cause: err}
	}

	req.Header.Set("User-Agent", launcherAgent)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: ".Auth", Value: cookie})
	}

	response, err := a.client.Do(req)
	if err != nil {
		return nil, &Error{Operation: "request", Reason: "HTTP transport failed", Cause: err}
	}

	data, readErr := io.ReadAll(io.LimitReader(response.Body, maxHTTPBytes+1))
	closeErr := response.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, &Error{Operation: "request", Reason: "cannot read HTTP response", Cause: err}
	}

	if response.StatusCode != http.StatusOK {
		return nil, &Error{Operation: "request", Reason: "HTTP status " + strconv.Itoa(response.StatusCode)}
	}

	if len(data) > maxHTTPBytes {
		return nil, &Error{Operation: "request", Reason: "HTTP response exceeds limit"}
	}

	return data, nil
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
		return nil, &Error{Operation: "login", Reason: "invalid compressed response", Cause: err}
	}

	result, readErr := io.ReadAll(io.LimitReader(reader, maxHTTPBytes+1))
	closeErr := reader.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, &Error{Operation: "login", Reason: "cannot decompress response", Cause: err}
	}

	if len(result) > maxHTTPBytes {
		return nil, &Error{Operation: "login", Reason: "decompressed response exceeds limit"}
	}

	return result, nil
}

func randomPassword() (string, error) {
	password := make([]byte, 6)
	for i := range password {
		value, err := rand.Int(rand.Reader, big.NewInt(26))
		if err != nil {
			return "", &Error{Operation: "login", Reason: "random generation failed", Cause: err}
		}

		password[i] = byte(value.Int64()) + 'a'
	}

	return string(password), nil
}
