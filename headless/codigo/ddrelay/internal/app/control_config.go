package app

import (
	"flag"
	"io"

	"ddbot/api/contract"
)

type ControlConfig struct {
	Socket     string
	Session    uint64
	Code       string
	Sessions   bool
	Operation  string
	RequestID  string
	Body       string
	PacketCode int32
	ClientID   int32
	Parameter1 int32
	Parameter2 int32
}

func parseControlConfig(args []string, stderr io.Writer) (ControlConfig, error) {
	flags := flag.NewFlagSet("control", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var c ControlConfig
	var evalFile, loadFile string
	var stop bool
	var packetCode, clientID, parameter1, parameter2 int
	flags.StringVar(&c.Socket, "socket", defaultControlPath(), "private relay control socket")
	flags.Uint64Var(&c.Session, "session", 0, "game session from -sessions")
	flags.StringVar(&c.Code, "eval", "", "Lua source for one remote evaluation")
	flags.StringVar(&evalFile, "eval-file", "", "file to upload for one evaluation")
	flags.StringVar(&loadFile, "load-script", "", "file to upload as session callbacks")
	flags.BoolVar(&stop, "stop-script", false, "stop callbacks in the selected session")
	flags.BoolVar(&c.Sessions, "sessions", false, "list game sessions")
	flags.StringVar(&c.RequestID, "request-id", "", "stable request ID for inspecting an uncertain result")
	flags.StringVar(&c.Body, "inject", "", "packet body hex for one raw injection")
	flags.IntVar(&packetCode, "packet-code", 0, "packet code for -inject")
	flags.IntVar(&clientID, "client-id", 0, "packet client id for -inject")
	flags.IntVar(&parameter1, "parameter1", 0, "packet parameter 1 for -inject")
	flags.IntVar(&parameter2, "parameter2", 0, "packet parameter 2 for -inject")
	if err := flags.Parse(args); err != nil {
		return c, err
	}

	c.PacketCode = int32(packetCode)
	c.ClientID = int32(clientID)
	c.Parameter1 = int32(parameter1)
	c.Parameter2 = int32(parameter2)

	count := 0

	for _, selected := range []bool{c.Code != "", evalFile != "", loadFile != "", stop, c.Sessions, c.Body != ""} {
		if selected {
			count++
		}
	}

	if count != 1 || flags.NArg() != 0 || c.Socket == "" {
		return c, &ProxyConfigError{Reason: "select exactly one of -sessions, -eval, -eval-file, -load-script, -stop-script or -inject"}
	}

	if c.Body != "" {
		if c.Session == 0 {
			return c, &ProxyConfigError{Reason: "a positive -session is required"}
		}

		if evalFile != "" || loadFile != "" || stop {
			return c, &ProxyConfigError{Reason: "-inject takes no script file"}
		}

		c.Operation = "inject"

		return c, nil
	}

	if c.Sessions {
		if c.Session != 0 {
			return c, &ProxyConfigError{Reason: "-sessions takes no session ID"}
		}

		c.Operation = "sessions"

		return c, nil
	}

	if c.Session == 0 {
		return c, &ProxyConfigError{Reason: "a positive -session is required"}
	}

	c.Operation = "eval"
	if stop {
		c.Operation = "stop"
	}

	path, limit := evalFile, contract.MaxEvalBytes
	if loadFile != "" {
		path, limit, c.Operation = loadFile, contract.MaxScriptBytes, "load"
	}

	if path != "" {
		source, err := readSource(path, limit)
		if err != nil {
			return c, err
		}

		c.Code = string(source)
	}

	return c, nil
}
