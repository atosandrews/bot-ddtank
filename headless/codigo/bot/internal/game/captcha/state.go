package captcha

type State struct {
	Revision    uint64
	Known       bool
	Required    bool
	Kind        int32
	DelayType   int32
	Prompt      string
	ImageBase64 string
	ImageSHA256 string
}
