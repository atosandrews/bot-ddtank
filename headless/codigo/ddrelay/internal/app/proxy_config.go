package app

type ProxyConfig struct {
	BotAddress, APIKey, RelayID                                       string
	BotTLS                                                            bool
	MaxSessions                                                       int
	Transparent, ManageRedirect                                       bool
	Ports                                                             []uint16
	ControlPath, Listen, Upstream, ScriptPath, RecordPath, CaptureDir string
	RecordAuto                                                        bool
}
