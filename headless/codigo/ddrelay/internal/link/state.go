package link

type State struct {
	ChangingScript   bool
	Diagnostics      Diagnostics
	BotSessionID     string
	Generation       uint64
	ScriptGeneration uint64
	Attached         bool
	Reason           string
}
