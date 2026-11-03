package script

import (
	"time"
	"unicode/utf8"

	lua "github.com/yuin/gopher-lua"

	"ddbot/bot/internal/game/action"
)

const maxReportPhaseBytes = 64
const maxReportReasonBytes = 1024

type StatusReport struct {
	Status    string
	Phase     string
	Reason    string
	ChangedAt time.Time
}

func (r *Runtime) StatusReport() *StatusReport {
	if r.report == nil {
		return nil
	}

	report := *r.report

	return &report
}

func (r *Runtime) reportStatus(l *lua.LState) int {
	a := readArguments(l)
	report := StatusReport{Status: a.text("status"), Phase: a.text("phase"), Reason: a.text("reason")}
	if err := a.finish(); err != nil {
		return fail(l, err)
	}

	switch report.Status {
	case "running", "waiting", "paused", "completed":
	default:
		return fail(l, &action.InvalidCommandError{Name: "session.report", Reason: "unsupported status: " + report.Status})
	}

	if report.Phase == "" || len(report.Phase) > maxReportPhaseBytes || !utf8.ValidString(report.Phase) {
		return fail(l, &action.InvalidCommandError{Name: "session.report", Reason: "phase must be UTF-8 with 1 to 64 bytes"})
	}

	if len(report.Reason) > maxReportReasonBytes || !utf8.ValidString(report.Reason) {
		return fail(l, &action.InvalidCommandError{Name: "session.report", Reason: "reason must be UTF-8 with at most 1024 bytes"})
	}

	previous := r.report
	if previous != nil && previous.Status == report.Status && previous.Phase == report.Phase && previous.Reason == report.Reason {
		return 0
	}

	report.ChangedAt = time.Now()
	r.report = &report

	return 0
}
