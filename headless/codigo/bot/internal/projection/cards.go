package projection

import (
	"ddbot/bot/internal/game"
	"ddbot/bot/internal/game/fight"
	"ddbot/bot/internal/game/value"
)

func projectCardRound(state *fight.State, f *fields) {
	cards := state.Cards.Reset()
	cards.Known = true
	cards.Type = f.integer("take_card_type")
	cards.HasNextMission = f.flag("has_next_mission")
	cards.CanEnterFinal = f.flag("can_enter_final")
	cards.Players = make(map[int32]fight.CardPlayer)

	for _, row := range f.list("players") {
		child := &fields{data: row}
		player := fight.CardPlayer{
			PlayerID:       child.integer("player_id"),
			Level:          child.integer("level"),
			Treatment:      child.integer("treatment"),
			GainedGP:       child.integer("gained_gp"),
			Winner:         child.flag("winner"),
			CardCount:      child.integer("card_count"),
			ResultArgument: child.integer("result_argument"),
			HasLevelAgain:  child.flag("has_level_again"),
			HasCardGet:     child.flag("has_card_get"),
		}
		cards.Players[player.PlayerID] = player
		if child.err != nil {
			f.err = child.err
		}
	}

	state.Cards = cards
}

func projectCards(next *game.Snapshot, f *fields) (string, error) {
	cards := &next.Fight.Cards

	switch f.integer("subcommand") {
	case 98:
		index := f.integer("card_index")
		system := f.flag("system_selected")
		if cards.Type == fight.CardsLarge && system {
			cards.SystemSelection = true
		}

		if index == 50 {
			return "card_selection_empty", nil
		}

		if index < 0 || index > 20 {
			return "", &FieldError{Name: "card_index"}
		}

		award := fight.CardAward{
			Index:          index,
			PlayerID:       f.integer("player_id"),
			LivingID:       f.integer("living_id"),
			SystemSelected: system,
			TemplateID:     f.integer("template_id"),
			Count:          f.integer("count"),
		}
		if _, exists := f.data["vip"]; exists {
			award.VIP = value.Of(f.flag("vip"))
		}
		if cards.Awards == nil {
			cards.Awards = make(map[int32]fight.CardAward)
		}
		cards.Awards[index] = award
		delete(cards.Revealed, index)
		delete(cards.Requested, index)
		return "card_awarded", nil
	case 89:
		revealed := make(map[int32]fight.CardReveal)
		for _, row := range f.list("cards") {
			child := &fields{data: row}
			index := child.integer("card_index")
			if index < 0 || index > 20 {
				return "", &FieldError{Name: "card_index"}
			}

			revealed[index] = fight.CardReveal{Index: index, TemplateID: child.integer("template_id"), Count: child.integer("count")}
			if child.err != nil {
				return "", child.err
			}
		}
		cards.Revealed = revealed
		cards.Closed = true
		return "cards_revealed", nil
	default:
		return "", &UnsupportedProjectionError{}
	}
}

func (p *Projector) projectCardRequest(decoded Decoded) (string, error) {
	next := p.session.Snapshot()
	f := &fields{data: decoded.Fields}
	sub := f.integer("subcommand")
	index := f.integer("card_index")
	kind := "normal"

	switch sub {
	case 98:
	case 114:
		kind = "extra"
	case 130:
		kind = "boss"
	default:
		return "", &UnsupportedProjectionError{}
	}

	if f.err != nil {
		return "", f.err
	}

	name := "card_requested"
	if index == 100 && sub != 114 {
		next.Fight.Cards.Finished = true
		name = "card_selection_finished"
	} else {
		if index < 0 || index > 20 {
			return "", &FieldError{Name: "card_index"}
		}

		if next.Fight.Cards.Requested == nil {
			next.Fight.Cards.Requested = make(map[int32]string)
		}

		next.Fight.Cards.Requested[index] = kind
	}

	if err := p.session.Commit(next); err != nil {
		return "", err
	}

	return name, nil
}
