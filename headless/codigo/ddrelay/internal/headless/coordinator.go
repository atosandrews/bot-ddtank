package headless

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CooldownError struct{ Until time.Time }

func (e *CooldownError) Error() string { return "headless authentication cooldown" }

type circuitState struct {
	Version  int       `json:"version"`
	Failures int       `json:"failures"`
	Until    time.Time `json:"until"`
}

// Coordinator serializes authentication probes and shares host cooldowns between
// Linux processes when dir is set. Separate machines require separate coordinators.
type Coordinator struct {
	directory string
	turn      chan struct{}
	states    map[string]circuitState
	now       func() time.Time
	jitter    func(time.Duration) time.Duration
}

func NewCoordinator(dir string) *Coordinator {
	return &Coordinator{
		directory: dir, turn: make(chan struct{}, 1), states: make(map[string]circuitState),
		now: time.Now, jitter: positiveJitter,
	}
}

// Do invokes fn once only when the host gate permits it. Waiting for another
// process is bounded by ctx; waiting for a scheduled probe returns immediately.
func (c *Coordinator) Do(ctx context.Context, host string, fn func(context.Context) error) (time.Time, error) {
	if err := ctx.Err(); err != nil {
		return time.Time{}, err
	}
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return time.Time{}, coordinationError("authentication host required", nil)
	}
	select {
	case <-ctx.Done():
		return time.Time{}, ctx.Err()
	case c.turn <- struct{}{}:
	}
	defer func() { <-c.turn }()
	state := c.states[host]
	path := ""
	if c.directory != "" {
		if err := privateDirectory(c.directory); err != nil {
			return time.Time{}, coordinationError("private coordination directory required", err)
		}
		digest := sha256.Sum256([]byte(host))
		base := filepath.Join(c.directory, hex.EncodeToString(digest[:]))
		unlock, err := lockCoordinator(ctx, base+".lock")
		if err != nil {
			return time.Time{}, err
		}
		defer unlock()
		path = base + ".json"
		state, err = readCircuit(path)
		if err != nil {
			return time.Time{}, coordinationError("cannot read authentication circuit", err)
		}
	}
	if err := ctx.Err(); err != nil {
		return time.Time{}, err
	}
	if c.now().Before(state.Until) {
		return state.Until, &CooldownError{Until: state.Until}
	}
	err := fn(ctx)
	if ctx.Err() != nil {
		return time.Time{}, ctx.Err()
	}
	if err == nil {
		state = circuitState{Version: 1}
	} else {
		var authError *Error
		if !errors.As(err, &authError) || (authError.Kind != "blocked" && authError.Kind != "challenge" && authError.Kind != "forbidden") {
			return time.Time{}, err
		}
		state.Version = 1
		state.Failures = min(state.Failures+1, 3)
		delay := c.jitter(time.Duration(state.Failures) * 5 * time.Minute)
		delay = max(delay, authError.RetryAfter)
		state.Until = c.now().UTC().Add(delay)
	}
	if path != "" {
		if writeErr := writePrivateJSON(path, state); writeErr != nil {
			return time.Time{}, coordinationError("cannot save authentication circuit", writeErr)
		}
	}
	c.states[host] = state
	return state.Until, err
}

func readCircuit(path string) (circuitState, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return circuitState{}, nil
	}
	if err != nil {
		return circuitState{}, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return circuitState{}, errors.New("private circuit file required")
	}
	file, err := os.Open(path)
	if err != nil {
		return circuitState{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil {
		return circuitState{}, err
	}
	if len(data) > 4096 {
		return circuitState{}, errors.New("circuit exceeds limit")
	}
	var state circuitState
	if err := json.Unmarshal(data, &state); err != nil {
		return circuitState{}, err
	}
	if state.Version != 1 || state.Failures < 0 || state.Failures > 3 || (state.Failures > 0 && state.Until.IsZero()) {
		return circuitState{}, errors.New("invalid circuit state")
	}
	return state, nil
}

func coordinationError(reason string, cause error) error {
	return &Error{Operation: "coordination", Kind: "config", Reason: reason, Cause: cause}
}

func positiveJitter(delay time.Duration) time.Duration {
	extra, err := rand.Int(rand.Reader, big.NewInt(int64(delay/10)+1))
	if err != nil {
		return delay
	}
	return delay + time.Duration(extra.Int64())
}
