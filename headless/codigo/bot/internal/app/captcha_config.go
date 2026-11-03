package app

type CaptchaConfig struct {
	Enabled           bool    `yaml:"enabled"`
	Library           string  `yaml:"library"`
	Model             string  `yaml:"model"`
	Charset           string  `yaml:"charset"`
	MinimumConfidence float64 `yaml:"minimum_confidence"`
	// Worker runs an external recognizer (program and arguments) instead of the built-in model.
	Worker []string `yaml:"worker"`
}
