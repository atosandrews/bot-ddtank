package game

import (
	"fmt"

	"ddbot/bot/internal/game/action"
	"ddbot/bot/internal/game/fight"
	"ddbot/bot/internal/game/room"
	"ddbot/bot/internal/game/value"
)

const MaxPendingCommands = 64

type Session struct {
	state    Snapshot
	writable bool
	nextID   uint64
	pending  []action.Request
	results  []action.Result
}

func NewSession(stream uint64, writable bool) *Session {
	return &Session{state: Snapshot{
		Stream: stream,
		Valid:  true,
		Fight:  fight.NewState(),
		Room:   room.NewState(),
	}, writable: writable}
}

func (s *Session) Snapshot() Snapshot { return s.state.Clone() }

func (s *Session) Commit(next Snapshot) error {
	if next.Stream != s.state.Stream || next.Revision != s.state.Revision {
		return &SessionError{Reason: "stale or foreign update"}
	}

	next.Revision++
	s.state = next.Clone()

	return nil
}

func (s *Session) CanSend() bool { return s.writable && s.state.Valid }

func (s *Session) Request(command action.Command) (uint64, error) {
	if err := s.validateCommand(command); err != nil {
		return 0, err
	}

	if len(s.pending) >= MaxPendingCommands {
		return 0, &action.UnavailableError{Reason: "command queue is full"}
	}

	if card, ok := command.(action.CardCommand); ok {
		if err := s.validateQueuedCard(card); err != nil {
			return 0, err
		}
	}

	s.nextID++
	request := action.Request{
		ID:              s.nextID,
		Command:         command,
		FightGeneration: s.state.Fight.Generation,
		TurnRevision:    s.state.Fight.TurnRevision,
		CardRevision:    s.state.Fight.Cards.Revision,
		RoomID:          value.Copy(s.state.Room.ID),
	}
	if s.state.Fight.Turn != nil {
		request.TurnNumber = value.Of(s.state.Fight.Turn.Number)
	}

	s.pending = append(s.pending, request)
	s.Report(action.Result{ID: request.ID, Name: command.Name(), Status: "queued"})

	return request.ID, nil
}

func (s *Session) validateCommand(command action.Command) error {
	if !s.CanSend() {
		return &action.UnavailableError{Reason: "session is read-only or state is invalid"}
	}

	if command == nil {
		return &action.InvalidCommandError{Reason: "command is nil"}
	}

	switch command.(type) {
	case action.CollectTemporaryRewards, action.UnlockBag, action.OrganizeInventory:
		if s.state.Player.ID == nil {
			return &action.UnavailableError{Reason: "player login is not confirmed"}
		}
	default:
	}

	if organize, ok := command.(action.OrganizeInventory); ok {
		return s.validateInventoryOrganization(organize)
	}

	if _, ok := command.(action.FriendAdd); ok && s.state.Player.ID == nil {
		return &action.UnavailableError{Reason: "player login is not confirmed"}
	}

	if _, ok := command.(action.RoomInvite); ok {
		if s.state.Player.ID == nil || s.state.Room.ID == nil || s.state.Room.Started != nil && *s.state.Room.Started {
			return &action.UnavailableError{Reason: "waiting room is not confirmed"}
		}
	}

	if answer, ok := command.(action.CaptchaAnswer); ok {
		challenge := s.state.Captcha
		if !challenge.Known || !challenge.Required || answer.Revision != challenge.Revision {
			return &action.UnavailableError{Reason: "CAPTCHA challenge is missing, cleared or replaced"}
		}
	}

	if _, ok := command.(action.RoomStart); ok && s.state.Captcha.Required {
		return &action.UnavailableError{Reason: "server verification is required before starting"}
	}

	if card, ok := command.(action.CardCommand); ok {
		return s.validateCard(card)
	}

	needsTurn, err := action.RequiresTurn(command)
	if err != nil {
		return err
	}

	if needsTurn {
		turn := s.state.Fight.Turn
		if s.state.Fight.Phase != "active" || turn == nil || s.state.Player.ID == nil {
			return &action.UnavailableError{Reason: "own active turn is not known"}
		}

		actor, exists := s.state.Fight.Actors[turn.ActorID]
		if !exists || actor.PlayerID == nil || *actor.PlayerID != *s.state.Player.ID {
			return &action.UnavailableError{Reason: "turn belongs to another actor"}
		}
	}

	return nil
}

func (s *Session) Validate(request action.Request) error {
	if err := s.validateCommand(request.Command); err != nil {
		return err
	}

	if _, ok := request.Command.(action.CardCommand); ok && (request.FightGeneration != s.state.Fight.Generation || request.CardRevision != s.state.Fight.Cards.Revision) {
		return &action.UnavailableError{Reason: "command belongs to an old card round"}
	}

	if _, ok := request.Command.(action.RoomInvite); ok && (request.RoomID == nil || s.state.Room.ID == nil || *request.RoomID != *s.state.Room.ID) {
		return &action.UnavailableError{Reason: "invitation belongs to another room"}
	}

	needsTurn, err := action.RequiresTurn(request.Command)
	if err != nil {
		return err
	}

	if needsTurn && (request.FightGeneration != s.state.Fight.Generation || request.TurnRevision != s.state.Fight.TurnRevision || request.TurnNumber == nil || *request.TurnNumber != s.state.Fight.Turn.Number) {
		return &action.UnavailableError{Reason: "command belongs to an old fight or turn"}
	}

	return nil
}

func (s *Session) Pending() []action.Request {
	pending := s.pending
	s.pending = nil

	return pending
}

func (s *Session) Report(result action.Result) {
	for i := range s.results {
		if s.results[i].ID == result.ID {
			s.results[i] = result

			return
		}
	}

	s.results = append(s.results, result)
	if len(s.results) > 128 {
		s.results = s.results[len(s.results)-128:]
	}
}

func (s *Session) Action(id uint64) (action.Result, error) {
	for _, result := range s.results {
		if result.ID == id {
			return result, nil
		}
	}

	return action.Result{}, &SessionError{Reason: fmt.Sprintf("action %d is not retained", id)}
}

func (s *Session) Invalidate(reason string) {
	s.state.Valid = false
	s.state.Fight.Cards.Closed = true
	s.state.Fight.Turn = nil
	s.state.Fight.Wind.Expire()
	s.state.Revision++

	for _, request := range s.pending {
		s.Report(action.Result{
			ID:     request.ID,
			Name:   request.Command.Name(),
			Status: "cancelled",
			Reason: reason,
		})
	}

	s.pending = nil
}

func (s *Session) LastActionID() uint64 { return s.nextID }

func (s *Session) PendingCount() int { return len(s.pending) }

func (s *Session) CancelAfter(id uint64, reason string) {
	retained := s.pending[:0]

	for _, request := range s.pending {
		if request.ID <= id {
			retained = append(retained, request)
			continue
		}

		s.Report(action.Result{
			ID:     request.ID,
			Name:   request.Command.Name(),
			Status: "cancelled",
			Reason: reason,
		})
	}

	s.pending = retained
}

func (s *Session) SetWritable(writable bool) { s.writable = writable }
