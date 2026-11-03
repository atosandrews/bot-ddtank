package app

import (
	"flag"
	"io"

	"ddbot/relay/internal/redirect"
)

func Run(args []string, output io.Writer) error {
	if len(args) == 0 {
		return &ProxyConfigError{Reason: "ddrelay run -config relay.yaml | login -config headless.yaml | control [flags]"}
	}

	switch args[0] {
	case "login":
		return runHeadless(args[1:], output)
	case "run":
		flags := flag.NewFlagSet("run", flag.ContinueOnError)
		flags.SetOutput(output)
		path := flags.String("config", "relay.yaml", "relay YAML configuration")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return &ProxyConfigError{Reason: "unexpected arguments"}
		}
		config, err := loadConfig(*path)
		if err != nil {
			return err
		}
		return runProxy(config, output)
	case "control":
		config, err := parseControlConfig(args[1:], output)
		if err != nil {
			return err
		}
		return runControl(config, output)
	case "__redirect-helper":
		if len(args) != 1 {
			return &ProxyConfigError{Reason: "helper takes no command-line arguments"}
		}
		return redirect.RunHelper()
	default:
		return &ProxyConfigError{Reason: "unknown command " + args[0]}
	}
}
