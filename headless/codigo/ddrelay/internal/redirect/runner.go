package redirect

import (
	"bytes"
	"context"
	"os/exec"
)

type Runner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type CommandRunner struct{ path string }

func NewCommandRunner() (*CommandRunner, error) {
	path, err := exec.LookPath("nft")
	if err != nil {
		return nil, &Error{Operation: "find nft", Cause: err}
	}

	return &CommandRunner{path: path}, nil
}

func (r *CommandRunner) Run(ctx context.Context, input string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, r.path, args...)
	command.Stdin = bytes.NewBufferString(input)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, &Error{Operation: "nft " + string(output), Cause: err}
	}

	return output, nil
}
