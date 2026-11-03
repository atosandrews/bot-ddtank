package app

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Bot struct {
		Address   string `yaml:"address"`
		APIKeyEnv string `yaml:"api_key_env"`
		TLS       bool   `yaml:"tls"`
	} `yaml:"bot"`
	Relay struct {
		ID             string   `yaml:"id"`
		Listen         string   `yaml:"listen"`
		Upstream       string   `yaml:"upstream"`
		Transparent    bool     `yaml:"transparent"`
		ManageRedirect bool     `yaml:"manage_redirect"`
		Ports          []uint16 `yaml:"ports"`
		Script         string   `yaml:"script"`
		MaxSessions    int      `yaml:"max_sessions"`
	} `yaml:"relay"`
	Control struct {
		Socket string `yaml:"socket"`
	} `yaml:"control"`
	Recording struct {
		Auto      bool   `yaml:"auto"`
		Path      string `yaml:"path"`
		Directory string `yaml:"directory"`
	} `yaml:"recording"`
}

func loadConfig(path string) (config ProxyConfig, result error) {
	var c Config
	c.Bot.TLS = true
	c.Bot.APIKeyEnv = "DDBOT_API_KEY"
	c.Relay.Listen = "127.0.0.1:11401"
	c.Relay.Ports = []uint16{1401}
	c.Relay.MaxSessions = 8
	c.Control.Socket = defaultControlPath()
	c.Recording.Directory = "captures"
	file, err := os.Open(path)
	if err != nil {
		return ProxyConfig{}, &ProxyConfigError{Reason: err.Error(), Cause: err}
	}

	defer func() {
		if err := file.Close(); err != nil {
			result = errors.Join(result, &ProxyConfigError{Reason: "close configuration", Cause: err})
		}
	}()
	data, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err != nil {
		return ProxyConfig{}, &ProxyConfigError{Reason: err.Error(), Cause: err}
	}

	if len(data) > 1<<20 {
		return ProxyConfig{}, &ProxyConfigError{Reason: "configuration exceeds 1 MiB"}
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&c); err != nil {
		return ProxyConfig{}, &ProxyConfigError{Reason: err.Error(), Cause: err}
	}

	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return ProxyConfig{}, &ProxyConfigError{Reason: "one YAML document required"}
	}

	for _, address := range []string{c.Bot.Address, c.Relay.Listen} {
		if err := validateAddress(address); err != nil {
			return ProxyConfig{}, err
		}
	}

	host, _, err := net.SplitHostPort(c.Bot.Address)
	if err != nil {
		return ProxyConfig{}, &ProxyConfigError{Reason: "invalid bot address", Cause: err}
	}

	if !c.Bot.TLS && (net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback()) {
		return ProxyConfig{}, &ProxyConfigError{Reason: "plaintext bot connection requires a loopback IP"}
	}

	key := os.Getenv(c.Bot.APIKeyEnv)
	if len(key) < 16 || len(key) > 1024 {
		return ProxyConfig{}, &ProxyConfigError{Reason: "API key environment variable must contain 16 to 1024 bytes"}
	}

	if c.Relay.MaxSessions < 1 || c.Relay.MaxSessions > 1024 {
		return ProxyConfig{}, &ProxyConfigError{Reason: "max_sessions must be between 1 and 1024"}
	}

	if c.Relay.Transparent {
		host, port, err := net.SplitHostPort(c.Relay.Listen)
		if err != nil {
			return ProxyConfig{}, &ProxyConfigError{Reason: "invalid relay listener", Cause: err}
		}

		if host != "127.0.0.1" || c.Relay.Upstream != "" || len(c.Relay.Ports) == 0 {
			return ProxyConfig{}, &ProxyConfigError{Reason: "transparent mode requires loopback, game ports and no fixed upstream"}
		}

		seen := make(map[uint16]bool)

		for _, p := range c.Relay.Ports {
			if p == 0 || fmt.Sprint(p) == port || seen[p] {
				return ProxyConfig{}, &ProxyConfigError{Reason: "invalid or repeated redirect port"}
			}

			seen[p] = true
		}
	} else {
		if c.Relay.ManageRedirect {
			return ProxyConfig{}, &ProxyConfigError{Reason: "managed redirect requires transparent mode"}
		}

		if err := validateAddress(c.Relay.Upstream); err != nil {
			return ProxyConfig{}, err
		}
	}

	if c.Relay.Listen == c.Relay.Upstream {
		return ProxyConfig{}, &ProxyConfigError{Reason: "listener cannot be upstream"}
	}

	if c.Control.Socket != "" && !filepath.IsAbs(c.Control.Socket) {
		return ProxyConfig{}, &ProxyConfigError{Reason: "absolute control socket path required"}
	}

	if c.Recording.Auto && c.Recording.Path != "" {
		return ProxyConfig{}, &ProxyConfigError{Reason: "recording auto and path are mutually exclusive"}
	}

	return ProxyConfig{
		BotAddress:     c.Bot.Address,
		BotTLS:         c.Bot.TLS,
		APIKey:         key,
		RelayID:        c.Relay.ID,
		MaxSessions:    c.Relay.MaxSessions,
		Listen:         c.Relay.Listen,
		Upstream:       c.Relay.Upstream,
		Transparent:    c.Relay.Transparent,
		ManageRedirect: c.Relay.ManageRedirect,
		Ports:          c.Relay.Ports,
		ScriptPath:     c.Relay.Script,
		ControlPath:    c.Control.Socket,
		RecordAuto:     c.Recording.Auto,
		RecordPath:     c.Recording.Path,
		CaptureDir:     c.Recording.Directory,
	}, nil
}

func validateAddress(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		return &ProxyConfigError{Reason: "host:port required"}
	}

	n, err := strconv.ParseUint(port, 10, 16)
	if err != nil || n == 0 {
		return &ProxyConfigError{Reason: "port must be between 1 and 65535"}
	}

	return nil
}

func defaultControlPath() string {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = fmt.Sprintf("/run/user/%d", os.Getuid())
	}

	return filepath.Join(base, "ddrelay", "control.sock")
}

func readSource(path string, limit int) (data []byte, result error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, &ProxyConfigError{Reason: err.Error(), Cause: err}
	}

	defer func() {
		if err := f.Close(); err != nil {
			result = errors.Join(result, &ProxyConfigError{Reason: "close script source", Cause: err})
		}
	}()
	source, err := io.ReadAll(io.LimitReader(f, int64(limit+1)))
	if err != nil {
		return nil, &ProxyConfigError{Reason: err.Error(), Cause: err}
	}

	if len(source) == 0 || len(source) > limit {
		return nil, &ProxyConfigError{Reason: "source is empty or exceeds size limit"}
	}

	return source, nil
}
