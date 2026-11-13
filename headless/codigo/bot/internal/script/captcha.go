package script

import (
	lua "github.com/yuin/gopher-lua"

	"ddbot/bot/internal/game/action"
)

func (r *Runtime) captchaBindings() map[string]lua.LGFunction {
	return map[string]lua.LGFunction{
		"solve": r.solveCaptcha,
		"state": func(l *lua.LState) int {
			state := r.session.View().Captcha
			if r.captchaJob != nil && r.captchaJob.revision != state.Revision {
				r.cancelCaptcha()
			}

			return r.push(map[string]any{
				"revision":     state.Revision,
				"known":        state.Known,
				"required":     state.Required,
				"kind":         state.Kind,
				"delay_type":   state.DelayType,
				"prompt":       state.Prompt,
				"image_base64": state.ImageBase64,
				"image_sha256": state.ImageSHA256,
			})
		},
		"answer": r.command(func(a *arguments) action.Command {
			revision := a.integer("revision", 54)
			if revision <= 0 {
				a.err = &action.InvalidCommandError{Name: "captcha.answer", Reason: "positive challenge revision required"}
			}

			return action.CaptchaAnswer{Revision: uint64(revision), Text: a.text("text")}
		}),
	}
}
