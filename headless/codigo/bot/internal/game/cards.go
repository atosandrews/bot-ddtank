package game

import (
	"ddbot/bot/internal/game/action"
	"ddbot/bot/internal/game/fight"
)

func (s *Session) validateCard(command action.CardCommand) error {
	cards := s.state.Fight.Cards
	if s.state.Player.ID == nil || !cards.Known || cards.Closed || cards.Finished || cards.SystemSelection || (s.state.Fight.Phase != "mission_ended" && s.state.Fight.Phase != "ended") {
		return &action.UnavailableError{Reason: "card selection is not open"}
	}

	if command.CardKind() == action.CardBoss && cards.Type != fight.CardsSmall || command.CardKind() != action.CardBoss && cards.Type != fight.CardsLarge {
		return &action.UnavailableError{Reason: "command does not match the card board"}
	}

	if _, exists := cards.Players[*s.state.Player.ID]; !exists {
		return &action.UnavailableError{Reason: "own card allowance is unknown"}
	}

	if command.CompletesCards() {
		return nil
	}

	limit := int32(20)
	if command.CardKind() == action.CardBoss {
		limit = 8
	}

	if command.CardIndex() < 0 || command.CardIndex() > limit {
		return &action.InvalidCommandError{Name: command.Name(), Reason: "card slot outside supported board"}
	}

	if _, exists := cards.Awards[command.CardIndex()]; exists {
		return &action.UnavailableError{Reason: "card has already been awarded"}
	}

	if _, exists := cards.Revealed[command.CardIndex()]; exists {
		return &action.UnavailableError{Reason: "card has already been revealed"}
	}

	if _, exists := cards.Requested[command.CardIndex()]; exists {
		return &action.UnavailableError{Reason: "card request outcome is pending"}
	}

	if command.CardKind() != action.CardExtra && s.remainingCards() <= 0 {
		return &action.UnavailableError{Reason: "normal card allowance is exhausted"}
	}

	return nil
}

func (s *Session) remainingCards() int32 {
	cards := s.state.Fight.Cards
	remaining := cards.Players[*s.state.Player.ID].CardCount

	for _, award := range cards.Awards {
		owner := s.state.Fight.CardOwner(award)
		if owner != nil && *owner == *s.state.Player.ID && (award.VIP == nil || !*award.VIP) {
			remaining--
		}
	}

	for _, kind := range cards.Requested {
		if kind != action.CardExtra {
			remaining--
		}
	}

	return remaining
}

func (s *Session) validateQueuedCard(command action.CardCommand) error {
	normalPending := int32(0)

	for _, request := range s.pending {
		card, ok := request.Command.(action.CardCommand)
		if !ok {
			continue
		}

		if card.CompletesCards() || !command.CompletesCards() && card.CardIndex() == command.CardIndex() {
			return &action.UnavailableError{Reason: "card selection already queued"}
		}

		if card.CardKind() != action.CardExtra {
			normalPending++
		}
	}

	if !command.CompletesCards() && command.CardKind() != action.CardExtra && normalPending >= s.remainingCards() {
		return &action.UnavailableError{Reason: "card allowance already queued"}
	}

	return nil
}
