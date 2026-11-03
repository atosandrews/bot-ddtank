package script

import "ddbot/bot/internal/captcha"

type captchaOutcome struct {
	result captcha.Result
	err    error
}
