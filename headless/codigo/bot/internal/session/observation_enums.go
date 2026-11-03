package session

import (
	"fmt"

	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/bot/internal/script"
)

func observedFightPhase(phase string) (pb.ObservedFightPhase, error) {
	switch phase {
	case "unknown":
		return pb.ObservedFightPhase_OBSERVED_FIGHT_PHASE_UNSPECIFIED, nil
	case "created":
		return pb.ObservedFightPhase_OBSERVED_FIGHT_PHASE_CREATED, nil
	case "loading":
		return pb.ObservedFightPhase_OBSERVED_FIGHT_PHASE_LOADING, nil
	case "active":
		return pb.ObservedFightPhase_OBSERVED_FIGHT_PHASE_ACTIVE, nil
	case "mission_ended":
		return pb.ObservedFightPhase_OBSERVED_FIGHT_PHASE_MISSION_ENDED, nil
	case "ended":
		return pb.ObservedFightPhase_OBSERVED_FIGHT_PHASE_ENDED, nil
	default:
		return 0, &Error{Code: "observation_state", Reason: "unknown fight phase: " + phase}
	}
}

func observedStrategyStatus(status string) (pb.StrategyStatus, error) {
	switch status {
	case "running":
		return pb.StrategyStatus_STRATEGY_STATUS_RUNNING, nil
	case "waiting":
		return pb.StrategyStatus_STRATEGY_STATUS_WAITING, nil
	case "paused":
		return pb.StrategyStatus_STRATEGY_STATUS_PAUSED, nil
	case "completed":
		return pb.StrategyStatus_STRATEGY_STATUS_COMPLETED, nil
	default:
		return 0, &Error{Code: "observation_state", Reason: "unknown strategy status: " + status}
	}
}

func observedRecognition(status script.RecognitionStatus) (pb.CaptchaRecognitionStatus, error) {
	switch status {
	case script.RecognitionIdle:
		return pb.CaptchaRecognitionStatus_CAPTCHA_RECOGNITION_STATUS_IDLE, nil
	case script.RecognitionDisabled:
		return pb.CaptchaRecognitionStatus_CAPTCHA_RECOGNITION_STATUS_DISABLED, nil
	case script.RecognitionPending:
		return pb.CaptchaRecognitionStatus_CAPTCHA_RECOGNITION_STATUS_PENDING, nil
	case script.RecognitionRecognized:
		return pb.CaptchaRecognitionStatus_CAPTCHA_RECOGNITION_STATUS_RECOGNIZED, nil
	case script.RecognitionUnrecognized:
		return pb.CaptchaRecognitionStatus_CAPTCHA_RECOGNITION_STATUS_UNRECOGNIZED, nil
	default:
		return 0, &Error{Code: "observation_state", Reason: fmt.Sprintf("unknown recognition status: %d", status)}
	}
}
