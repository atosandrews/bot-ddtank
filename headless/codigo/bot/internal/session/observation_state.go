package session

import (
	"cmp"
	"slices"
	"time"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func (s *Session) observation() (*pb.GetSessionObservationResponse, error) {
	state := s.state.Snapshot()
	if len(state.Fight.Actors) > maxObservedActors {
		return nil, &Error{Code: "observation_limit", Reason: "fight actor count exceeds observation limit"}
	}

	phase, err := observedFightPhase(state.Fight.Phase)
	if err != nil {
		return nil, err
	}

	recognition, err := observedRecognition(s.runtime.RecognitionStatus(state.Captcha.Revision, state.Captcha.Required))
	if err != nil {
		return nil, err
	}

	if !state.Captcha.Known {
		recognition = pb.CaptchaRecognitionStatus_CAPTCHA_RECOGNITION_STATUS_UNSPECIFIED
	}

	script, err := s.observeScript()
	if err != nil {
		return nil, err
	}

	cards := state.Fight.Cards
	result := &pb.GetSessionObservationResponse{
		BotSessionId:        s.id,
		ObservedAtUnixMilli: time.Now().UnixMilli(),
		AppliedSequence:     s.sequence,
		StateRevision:       state.Revision,
		BindingGeneration:   s.generation,
		ConnectionState:     pb.ObservationConnectionState_OBSERVATION_CONNECTION_STATE_DETACHED,
		Initialized:         s.initialized,
		Valid:               state.Valid,
		CanSend:             s.state.CanSend(),
		PlayerId:            state.Player.ID,
		Script:              script,
		Room: &pb.RoomObservation{
			Id:                     state.Room.ID,
			InstanceId:             state.Room.MapID,
			Difficulty:             state.Room.Difficulty,
			Started:                state.Room.Started,
			StartCancelledRevision: state.Room.StartCancelledRevision,
		},
		Fight: &pb.FightObservation{
			Phase:        phase,
			Generation:   state.Fight.Generation,
			TurnRevision: state.Fight.TurnRevision,
			MapId:        state.Fight.MapID,
			MissionId:    state.Fight.MissionID,
			Rewards: &pb.RewardObservation{
				Known:               cards.Known,
				Revision:            cards.Revision,
				CompletionRequested: cards.Finished,
				Closed:              cards.Closed,
				HasNextMission:      cards.HasNextMission,
				CanEnterFinal:       cards.CanEnterFinal,
				AwardsObserved:      uint32(len(cards.Awards)),
				SelectionsPending:   uint32(len(cards.Requested)),
			},
		},
		Captcha: &pb.CaptchaObservation{
			Known:       state.Captcha.Known,
			Required:    state.Captcha.Required,
			Revision:    state.Captcha.Revision,
			Recognition: recognition,
		},
		Control: &pb.ControlObservation{
			PendingCommands:  uint32(len(s.pending) + s.state.PendingCount()),
			RequestEntries:   uint32(len(s.requests)),
			RequestCapacity:  maxScriptRequests,
			ResultCacheBytes: uint64(s.resultBytes),
		},
	}
	if s.attached {
		result.ConnectionState = pb.ObservationConnectionState_OBSERVATION_CONNECTION_STATE_ATTACHED
	}

	if turn := state.Fight.Turn; turn != nil {
		result.Fight.Turn = &pb.TurnObservation{ActorId: turn.ActorID, Number: turn.Number}
	}

	for _, actor := range state.Fight.Actors {
		result.Fight.Actors = append(result.Fight.Actors, &pb.ActorObservation{
			Id:        actor.ID,
			PlayerId:  actor.PlayerID,
			Team:      actor.Team,
			Hp:        actor.HP,
			PendingHp: actor.PendingHP,
			Alive:     actor.Alive,
		})
	}

	slices.SortFunc(result.Fight.Actors, func(a, b *pb.ActorObservation) int {
		return cmp.Compare(a.Id, b.Id)
	})

	return result, nil
}

func (s *Session) observeScript() (*pb.ScriptObservation, error) {
	result := &pb.ScriptObservation{
		Generation: s.scriptGeneration,
		Lifecycle:  pb.ScriptLifecycle_SCRIPT_LIFECYCLE_STOPPED,
	}
	if s.scriptLoaded {
		result.Lifecycle = pb.ScriptLifecycle_SCRIPT_LIFECYCLE_LOADED
	}

	report := s.runtime.StatusReport()
	if report == nil {
		return result, nil
	}

	status, err := observedStrategyStatus(report.Status)
	if err != nil {
		return nil, err
	}

	result.Report = &pb.StrategyReport{
		Status:             status,
		Phase:              report.Phase,
		Reason:             report.Reason,
		ChangedAtUnixMilli: report.ChangedAt.UnixMilli(),
	}

	return result, nil
}
