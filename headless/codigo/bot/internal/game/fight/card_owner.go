package fight

import "ddbot/bot/internal/game/value"

func (s State) CardOwner(award CardAward) *int32 {
	switch s.Cards.Type {
	case CardsLarge:
		if _, exists := s.Cards.Players[award.PlayerID]; exists {
			return value.Of(award.PlayerID)
		}
	case CardsSmall:
	default:
		return nil
	}

	actor, exists := s.Actors[award.LivingID]
	if !exists {
		return nil
	}

	return value.Copy(actor.PlayerID)
}
