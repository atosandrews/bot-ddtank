package action

type CaptchaAnswer struct {
	Revision uint64
	Text     string
}

func (CaptchaAnswer) Name() string { return "captcha_answer" }
