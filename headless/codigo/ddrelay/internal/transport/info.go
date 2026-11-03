package transport

import "ddbot/relay/internal/link"

type Info struct {
	Diagnostics       link.Diagnostics `json:"bot_diagnostics"`
	EventSequence     uint64           `json:"event_sequence"`
	StaleRejections   uint64           `json:"stale_rejections"`
	UncertainCommands uint64           `json:"uncertain_commands"`
	Session           uint64           `json:"session"`
	Client            string           `json:"client"`
	Upstream          string           `json:"upstream"`
	CipherReady       bool             `json:"cipher_ready"`
	BotAttached       bool             `json:"bot_attached"`
	GameReady         bool             `json:"game_ready"`
	BotSessionID      string           `json:"bot_session_id"`
	Generation        uint64           `json:"binding_generation"`
	Reason            string           `json:"reason,omitempty"`
}
