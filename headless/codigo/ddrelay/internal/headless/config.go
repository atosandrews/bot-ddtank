package headless

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

const maxConfigBytes = 65536

type Config struct {
	SessionFile string      `yaml:"session_file"`
	Characters  []Character `yaml:"characters"`
	Auth        AuthConfig  `yaml:"auth"`
}

type AuthConfig struct {
	Transport       string `yaml:"transport"`
	StatusFile      string `yaml:"status_file"`
	CoordinationDir string `yaml:"coordination_dir"`
}

func LoadConfig(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, &Error{Operation: "config", Kind: "config", Reason: "cannot read configuration", Cause: err}
	}

	data, readErr := io.ReadAll(io.LimitReader(file, maxConfigBytes+1))
	if err := errors.Join(readErr, file.Close()); err != nil {
		return Config{}, &Error{Operation: "config", Kind: "config", Reason: "cannot read configuration", Cause: err}
	}

	if len(data) > maxConfigBytes {
		return Config{}, &Error{Operation: "config", Kind: "config", Reason: "configuration exceeds limit"}
	}

	var config Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&config); err != nil {
		return Config{}, &Error{Operation: "config", Kind: "config", Reason: "invalid configuration", Cause: err}
	}

	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return Config{}, &Error{Operation: "config", Kind: "config", Reason: "one document required"}
	}

	if config.SessionFile == "" {
		return Config{}, &Error{Operation: "config", Kind: "config", Reason: "account session file required"}
	}

	if !filepath.IsAbs(config.SessionFile) {
		config.SessionFile = filepath.Join(filepath.Dir(path), config.SessionFile)
	}
	if config.Auth.Transport == "" {
		config.Auth.Transport = "standard"
	}
	if config.Auth.Transport != "standard" && config.Auth.Transport != "launcher_chromium87" {
		return Config{}, &Error{Operation: "config", Kind: "config", Reason: "unsupported authentication transport"}
	}
	for _, value := range []*string{&config.Auth.StatusFile, &config.Auth.CoordinationDir} {
		if *value != "" && !filepath.IsAbs(*value) {
			*value = filepath.Join(filepath.Dir(path), *value)
		}
	}

	if len(config.Characters) == 0 || len(config.Characters) > 16 {
		return Config{}, &Error{Operation: "config", Kind: "config", Reason: "one to sixteen characters required"}
	}

	ids := make(map[int32]bool)
	websiteIDs := make(map[int32]bool)
	for _, character := range config.Characters {
		if character.ID <= 0 || character.WebsiteID <= 0 || character.Name == "" || ids[character.ID] || websiteIDs[character.WebsiteID] {
			return Config{}, &Error{Operation: "config", Kind: "config", Reason: "invalid or repeated character"}
		}

		ids[character.ID] = true
		websiteIDs[character.WebsiteID] = true
		host, port, err := net.SplitHostPort(character.Upstream)
		if err != nil || host == "" {
			return Config{}, &Error{Operation: "config", Kind: "config", Reason: "invalid game address"}
		}

		if value, err := strconv.ParseUint(port, 10, 16); err != nil || value == 0 {
			return Config{}, &Error{Operation: "config", Kind: "config", Reason: "invalid game port"}
		}
	}

	return config, nil
}

func LoadAccountSession(path string) (AccountSession, error) {
	file, err := os.Open(path)
	if err != nil {
		return AccountSession{}, &Error{Operation: "account session", Kind: "session", Reason: "cannot open session file", Cause: err}
	}

	info, statErr := file.Stat()
	data, readErr := io.ReadAll(io.LimitReader(file, maxConfigBytes+1))
	closeErr := file.Close()
	if err := errors.Join(statErr, readErr, closeErr); err != nil {
		return AccountSession{}, &Error{Operation: "account session", Kind: "session", Reason: "cannot read session file", Cause: err}
	}

	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || len(data) > maxConfigBytes {
		return AccountSession{}, &Error{Operation: "account session", Kind: "session", Reason: "private regular session file required"}
	}

	var session AccountSession
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&session); err != nil {
		return AccountSession{}, &Error{Operation: "account session", Kind: "session", Reason: "invalid session file", Cause: err}
	}

	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return AccountSession{}, &Error{Operation: "account session", Kind: "session", Reason: "one session required"}
	}

	if len(session.Cookie) < 16 || len(session.Cookie) > 8192 {
		return AccountSession{}, &Error{Operation: "account session", Kind: "session", Reason: "invalid account cookie"}
	}

	cookie := http.Cookie{Name: ".Auth", Value: session.Cookie}
	if err := cookie.Valid(); err != nil {
		return AccountSession{}, &Error{Operation: "account session", Kind: "session", Reason: "invalid account cookie", Cause: err}
	}

	return session, nil
}
