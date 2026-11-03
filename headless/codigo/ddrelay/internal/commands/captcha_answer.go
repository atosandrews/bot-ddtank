package commands

type CaptchaAnswer struct {
	Text string
}

func (CaptchaAnswer) Name() string { return "captcha_answer" }
