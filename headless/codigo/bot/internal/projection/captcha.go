package projection

import (
	"ddbot/bot/internal/game"
	"ddbot/bot/internal/game/captcha"
)

func projectCaptcha(next *game.Snapshot, f *fields) (string, error) {
	state := captcha.State{
		Revision: next.Captcha.Revision + 1,
		Known:    true,
		Required: f.flag("required"),
		Kind:     f.integer("kind"),
	}
	name := "captcha_cleared"
	if state.Required {
		state.DelayType = f.integer("delay_type")
		state.Prompt = f.text("prompt")
		state.ImageBase64 = f.text("image_base64")
		state.ImageSHA256 = f.text("image_sha256")
		name = "captcha_required"
	}

	next.Captcha = state

	return name, f.err
}
