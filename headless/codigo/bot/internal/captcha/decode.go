package captcha

import (
	"math"
	"strings"
)

func decode(values []float32, classes []string, minimum float64) (Result, error) {
	if len(values) == 0 || len(values)%len(classes) != 0 {
		return Result{}, &Error{Operation: "decode", Reason: "unexpected model output shape"}
	}

	text := strings.Builder{}
	confidence := 1.0
	previous := 0
	peak := 0.0
	for offset := 0; offset < len(values); offset += len(classes) {
		row := values[offset : offset+len(classes)]
		best := 0
		for i, v := range row {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				return Result{}, &Error{Operation: "decode", Reason: "non-finite model output"}
			}
			if v > row[best] {
				best = i
			}
		}

		if best != previous && previous != 0 {
			text.WriteString(classes[previous])
			confidence = min(confidence, peak)
		}
		if best != previous {
			peak = 0
		}
		if best != 0 {
			sum := 0.0
			for _, v := range row {
				sum += math.Exp(float64(v - row[best]))
			}
			peak = max(peak, 1/sum)
		}
		previous = best
	}

	if previous != 0 {
		text.WriteString(classes[previous])
		confidence = min(confidence, peak)
	}
	answer := text.String()
	if len(answer) != 4 || strings.Trim(answer, "0123456789") != "" {
		return Result{Confidence: confidence}, &Error{Operation: "recognize", Reason: "model did not identify four digits"}
	}

	if confidence < minimum {
		return Result{Confidence: confidence}, &Error{Operation: "recognize", Reason: "recognition confidence is below the configured threshold"}
	}

	return Result{Text: answer, Confidence: confidence}, nil
}
