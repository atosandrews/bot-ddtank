package script

import (
	"context"
	"sync/atomic"
	"time"

	lua "github.com/yuin/gopher-lua"

	"ddbot/bot/internal/game/action"
)

const captchaSolveTimeout = 10 * time.Second

type captchaJob struct {
	revision uint64
	cancel   context.CancelFunc
	outcome  atomic.Pointer[captchaOutcome]
}

func (r *Runtime) cancelCaptcha() {
	if r.captchaJob != nil {
		r.captchaJob.cancel()
		r.captchaJob = nil
	}
}

func (r *Runtime) solveCaptcha(l *lua.LState) int {
	if l.GetTop() != 0 {
		return fail(l, &action.InvalidCommandError{Name: "captcha.solve", Reason: "expected no arguments"})
	}

	challenge := r.session.View().Captcha
	if !challenge.Known || !challenge.Required {
		r.cancelCaptcha()

		return fail(l, &action.UnavailableError{Reason: "no current CAPTCHA image is available"})
	}

	if r.captchaSolver == nil {
		return r.push(map[string]any{"revision": challenge.Revision, "status": "disabled", "reason": "configure Bot CAPTCHA recognition"})
	}

	if r.captchaJob != nil && r.captchaJob.revision != challenge.Revision {
		r.cancelCaptcha()
	}

	if r.captchaJob == nil {
		ctx, cancel := context.WithTimeout(context.Background(), captchaSolveTimeout)
		job := &captchaJob{revision: challenge.Revision, cancel: cancel}
		r.captchaJob = job
		solver := r.captchaSolver
		go func() {
			defer cancel()
			result, err := solver.Solve(ctx, challenge.ImageBase64)
			job.outcome.Store(&captchaOutcome{result: result, err: err})
		}()
	}

	job := r.captchaJob
	outcome := job.outcome.Load()
	if outcome == nil {
		return r.push(map[string]any{"revision": job.revision, "status": "pending"})
	}

	if outcome.err != nil {
		return r.push(map[string]any{"revision": job.revision, "status": "unrecognized", "reason": outcome.err.Error(), "confidence": outcome.result.Confidence})
	}

	return r.push(map[string]any{"revision": job.revision, "status": "solved", "text": outcome.result.Text, "confidence": outcome.result.Confidence})
}
