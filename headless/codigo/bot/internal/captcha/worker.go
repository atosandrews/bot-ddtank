package captcha

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"sync"
	"time"
)

const workerStartTimeout = 120 * time.Second

// Worker delegates recognition to a local process that answers one JSON line per request,
// such as the bot-ddtank OCR with its preprocessing variants and two-model vote.
type Worker struct {
	mutex   sync.Mutex
	command *exec.Cmd
	input   io.WriteCloser
	output  *bufio.Reader
	closed  bool
}

type workerReply struct {
	Ready      bool    `json:"ready"`
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Error      string  `json:"error"`
}

func NewWorker(program string, args ...string) (*Worker, error) {
	if program == "" {
		return nil, &Error{Operation: "configure", Reason: "worker command is required"}
	}

	command := exec.Command(program, args...)
	command.Stderr = os.Stderr
	input, err := command.StdinPipe()
	if err != nil {
		return nil, &Error{Operation: "initialize", Reason: "cannot open worker input", Cause: err}
	}

	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, &Error{Operation: "initialize", Reason: "cannot open worker output", Cause: err}
	}

	if err := command.Start(); err != nil {
		return nil, &Error{Operation: "initialize", Reason: "cannot start worker", Cause: err}
	}

	worker := &Worker{command: command, input: input, output: bufio.NewReader(stdout)}
	ready := make(chan error, 1)
	go func() {
		reply, err := worker.read()
		if err == nil && !reply.Ready {
			err = errors.New("unexpected first reply")
		}
		ready <- err
	}()

	select {
	case err = <-ready:
	case <-time.After(workerStartTimeout):
		err = errors.New("worker did not become ready")
	}

	if err != nil {
		return nil, &Error{Operation: "initialize", Reason: "worker failed to load its models", Cause: errors.Join(err, worker.Close())}
	}

	return worker, nil
}

func (w *Worker) Solve(ctx context.Context, encoded string) (result Result, failure error) {
	defer func() {
		if failure != nil {
			failure = &Error{Operation: "solve", Reason: failure.Error(), Cause: failure}
		}
	}()

	if err := ctx.Err(); err != nil {
		return result, &Error{Operation: "recognize", Reason: "recognition cancelled", Cause: err}
	}

	if len(encoded) == 0 || len(encoded) > 88<<10 {
		return result, &Error{Operation: "image", Reason: "image exceeds the game packet size"}
	}

	w.mutex.Lock()
	defer w.mutex.Unlock()
	if w.closed {
		return result, &Error{Operation: "recognize", Reason: "worker is closed"}
	}

	request, err := json.Marshal(map[string]string{"image": encoded})
	if err != nil {
		return result, err
	}

	if _, err := w.input.Write(append(request, '\n')); err != nil {
		return result, &Error{Operation: "recognize", Reason: "cannot send image to worker", Cause: err}
	}

	reply, err := w.read()
	if err != nil {
		return result, &Error{Operation: "recognize", Reason: "worker reply unavailable", Cause: err}
	}

	if reply.Error != "" {
		return result, &Error{Operation: "recognize", Reason: reply.Error}
	}

	if len(reply.Text) != 4 || math.IsNaN(reply.Confidence) || reply.Confidence < 0 || reply.Confidence > 1 {
		return result, &Error{Operation: "recognize", Reason: "worker did not return four digits"}
	}

	for _, digit := range reply.Text {
		if digit < '0' || digit > '9' {
			return result, &Error{Operation: "recognize", Reason: "worker did not return four digits"}
		}
	}

	return Result{Text: reply.Text, Confidence: reply.Confidence}, nil
}

func (w *Worker) read() (workerReply, error) {
	var reply workerReply
	line, err := w.output.ReadBytes('\n')
	if err != nil {
		return reply, err
	}

	if err := json.Unmarshal(line, &reply); err != nil {
		return reply, err
	}

	return reply, nil
}

func (w *Worker) Close() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	if w.closed {
		return nil
	}

	w.closed = true
	closeErr := w.input.Close()
	waitErr := w.command.Wait()
	var exit *exec.ExitError
	if errors.As(waitErr, &exit) {
		waitErr = nil
	}

	if err := errors.Join(closeErr, waitErr); err != nil {
		return &Error{Operation: "close", Reason: "cannot stop worker", Cause: err}
	}

	return nil
}
