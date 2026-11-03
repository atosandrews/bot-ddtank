package app

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Captcha      CaptchaConfig  `yaml:"captcha"`
	Listen       string         `yaml:"listen"`
	LuaDirectory string         `yaml:"lua_dir"`
	TLS          TLSConfig      `yaml:"tls"`
	Tenants      []TenantConfig `yaml:"tenants"`
	MaxSessions  int            `yaml:"max_sessions"`
	PerTenant    int            `yaml:"sessions_per_tenant"`
	PerRelay     int            `yaml:"sessions_per_relay"`
	Grace        time.Duration  `yaml:"reconnect_grace"`
	Timeout      time.Duration  `yaml:"lua_timeout"`
}

func LoadConfig(path string) (config Config, keys map[string]string, result error) {
	c := Config{
		Captcha:      CaptchaConfig{Model: "models/captcha/model.onnx", Charset: "models/captcha/charset.json", MinimumConfidence: 0.75},
		Listen:       "127.0.0.1:7443",
		LuaDirectory: "lua",
		MaxSessions:  64,
		PerTenant:    16,
		PerRelay:     8,
		Grace:        120 * time.Second,
		Timeout:      time.Second,
	}
	file, err := os.Open(path)
	if err != nil {
		return c, nil, &ConfigurationError{Field: "config_open", Cause: err, Reason: err.Error()}
	}

	defer func() {
		if err := file.Close(); err != nil {
			result = errors.Join(result, &ConfigurationError{Field: "close configuration", Cause: err})
		}
	}()
	data, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err != nil {
		return c, nil, &ConfigurationError{Field: "config_read", Cause: err, Reason: err.Error()}
	}

	if len(data) > 1<<20 {
		return c, nil, &ConfigurationError{Field: "config_size", Reason: "configuration exceeds 1 MiB"}
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&c); err != nil {
		return c, nil, &ConfigurationError{Field: "config_decode", Cause: err, Reason: err.Error()}
	}

	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return c, nil, &ConfigurationError{Field: "config_decode", Reason: "one YAML document required"}
	}

	host, _, err := net.SplitHostPort(c.Listen)
	if err != nil {
		return c, nil, &ConfigurationError{Field: "config_listen", Cause: err, Reason: err.Error()}
	}

	if c.TLS.Cert == "" || c.TLS.Key == "" {
		if c.TLS.Cert != "" || c.TLS.Key != "" || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
			return c, nil, &ConfigurationError{Field: "config_tls", Reason: "TLS certificate and key required outside loopback"}
		}
	}

	if c.MaxSessions < 1 || c.PerTenant < 1 || c.PerRelay < 1 || c.Grace <= 0 || c.Timeout <= 0 {
		return c, nil, &ConfigurationError{Field: "config_limits", Reason: "positive finite limits required"}
	}

	keys = make(map[string]string)

	for _, t := range c.Tenants {
		if t.ID == "" || len(t.ID) > 128 || t.APIKeyEnv == "" {
			return c, nil, &ConfigurationError{Field: "config_tenant", Reason: "tenant ID and key environment variable required"}
		}

		key := os.Getenv(t.APIKeyEnv)
		if len(key) < 16 || len(key) > 1024 {
			return c, nil, &ConfigurationError{Field: "config_key", Reason: "API key environment variable must contain 16 to 1024 bytes"}
		}

		if _, exists := keys[key]; exists {
			return c, nil, &ConfigurationError{Field: "config_key", Reason: "duplicate API key"}
		}

		keys[key] = t.ID
	}

	if len(keys) == 0 {
		return c, nil, &ConfigurationError{Field: "config_tenants", Reason: "at least one tenant required"}
	}

	return c, keys, nil
}
