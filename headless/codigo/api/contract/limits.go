package contract

const (
	APIVersion      = 1
	CodecRevision   = "ddclassico-20261021"
	MaxMessageBytes = 2 << 20
	MaxEvalBytes    = 64 << 10
	MaxScriptBytes  = 1 << 20
	MaxResultBytes  = 256 << 10
	MaxCommands     = 64
	MaxLogBytes     = 8 << 10
	MaxEvents       = 4096
	MaxHistoryBytes = 16 << 20
	MaxDepth        = 32
)
