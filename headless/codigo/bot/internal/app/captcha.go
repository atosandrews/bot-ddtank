package app

import (
	"context"
	"os"

	"go.uber.org/fx"

	"ddbot/bot/internal/captcha"
)

func loadCaptcha(lifecycle fx.Lifecycle, config CaptchaConfig) (captcha.Solver, error) {
	if len(config.Worker) > 0 {
		worker, err := captcha.NewWorker(config.Worker[0], config.Worker[1:]...)
		if err != nil {
			return nil, err
		}

		lifecycle.Append(fx.Hook{OnStop: func(context.Context) error { return worker.Close() }})

		return worker, nil
	}

	library := config.Library
	if library == "" {
		library = os.Getenv("DDBOT_ONNXRUNTIME_LIBRARY")
	}
	if library == "" {
		library = "libonnxruntime.so"
	}

	recognizer, err := captcha.NewRecognizer(library, config.Model, config.Charset, config.MinimumConfidence)
	if err != nil {
		return nil, err
	}

	lifecycle.Append(fx.Hook{OnStop: func(context.Context) error { return recognizer.Close() }})

	return recognizer, nil
}
