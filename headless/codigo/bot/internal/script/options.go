package script

import "ddbot/bot/internal/captcha"

type Options struct {
	CaptchaSolver captcha.Solver
	Modules       []Module
	Library       []Module
}
