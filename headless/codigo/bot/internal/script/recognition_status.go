package script

const (
	RecognitionIdle RecognitionStatus = iota
	RecognitionDisabled
	RecognitionPending
	RecognitionRecognized
	RecognitionUnrecognized
)

type RecognitionStatus uint8

func (r *Runtime) RecognitionStatus(revision uint64, required bool) RecognitionStatus {
	if !required {
		return RecognitionIdle
	}

	if r.captchaSolver == nil {
		return RecognitionDisabled
	}

	job := r.captchaJob
	if job == nil || job.revision != revision {
		return RecognitionIdle
	}

	outcome := job.outcome.Load()
	if outcome == nil {
		return RecognitionPending
	}

	if outcome.err != nil {
		return RecognitionUnrecognized
	}

	return RecognitionRecognized
}
