package redirect

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Manager struct {
	config Config
	table  string
	runner Runner
	owned  bool
}

func NewManager(config Config, token string, runner Runner) (*Manager, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	decoded, err := hex.DecodeString(token)
	if err != nil || len(decoded) != 16 {
		return nil, &Error{Operation: "token", Reason: "expected 128-bit hexadecimal token"}
	}

	return &Manager{config: config, table: fmt.Sprintf("ddtank_%d_%d_%s", config.UID, config.Port, token), runner: runner}, nil
}

func (m *Manager) Table() string { return m.table }

func (m *Manager) exists(ctx context.Context) (bool, error) {
	output, err := m.runner.Run(ctx, "", "-j", "list", "tables")
	if err != nil {
		return false, err
	}

	var list struct {
		Entries []struct {
			Table *struct {
				Family string `json:"family"`
				Name   string `json:"name"`
			} `json:"table"`
		} `json:"nftables"`
	}
	if err := json.Unmarshal(output, &list); err != nil {
		return false, &Error{Operation: "list tables", Cause: err}
	}

	for _, entry := range list.Entries {
		if entry.Table != nil && entry.Table.Family == "ip" && entry.Table.Name == m.table {
			return true, nil
		}
	}

	return false, nil
}

func (m *Manager) Install(ctx context.Context) error {
	present, err := m.exists(ctx)
	if err != nil {
		return err
	}

	if present {
		return &Error{Operation: "install", Reason: "table already exists: " + m.table}
	}

	ports := make([]string, 0, len(m.config.Ports))

	for _, port := range m.config.Ports {
		ports = append(ports, strconv.Itoa(int(port)))
	}

	transaction := fmt.Sprintf("add table ip %s\nadd chain ip %s output { type nat hook output priority dstnat; policy accept; }\nadd rule ip %s output meta skuid %d meta mark != %d tcp dport { %s } counter redirect to :%d\n", m.table, m.table, m.table, m.config.UID, m.config.Mark, strings.Join(ports, ", "), m.config.Port)
	m.owned = true
	if _, err := m.runner.Run(ctx, transaction, "-f", "-"); err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		return errors.Join(err, m.Close(cleanup))
	}

	return nil
}

func (m *Manager) Close(ctx context.Context) error {
	if !m.owned {
		return nil
	}

	var last error

	for attempt := 0; attempt < 3; attempt++ {
		present, err := m.exists(ctx)
		if err == nil && !present {
			m.owned = false

			return nil
		}

		if err != nil {
			last = err
			continue
		}

		if _, err := m.runner.Run(ctx, "delete table ip "+m.table+"\n", "-f", "-"); err != nil {
			last = err
			continue
		}

		present, err = m.exists(ctx)
		if err == nil && !present {
			m.owned = false

			return nil
		}

		last = err
	}

	return &Error{Operation: "cleanup; run sudo nft delete table ip " + m.table, Cause: last, Reason: "table removal could not be confirmed"}
}
