package script

import (
	"slices"

	"ddbot/bot/internal/game/fight"
)

func (r *Runtime) cardState() map[string]any {
	state := r.session.View().Fight
	cards := state.Cards
	players := make([]map[string]any, 0, len(cards.Players))
	ids := make([]int32, 0, len(cards.Players))

	for id := range cards.Players {
		ids = append(ids, id)
	}

	slices.Sort(ids)

	for _, id := range ids {
		p := cards.Players[id]
		players = append(players, map[string]any{
			"player_id":       p.PlayerID,
			"level":           p.Level,
			"treatment":       p.Treatment,
			"gained_gp":       p.GainedGP,
			"winner":          p.Winner,
			"card_count":      p.CardCount,
			"result_argument": p.ResultArgument,
			"has_level_again": p.HasLevelAgain,
			"has_card_get":    p.HasCardGet,
		})
	}

	awards := make([]map[string]any, 0, len(cards.Awards))
	revealed := make([]map[string]any, 0, len(cards.Revealed))
	requested := make([]map[string]any, 0, len(cards.Requested))
	available := make([]int32, 0)
	count := int32(0)
	if cards.Known && cards.Type == fight.CardsSmall {
		count = 9
	}

	if cards.Known && cards.Type == fight.CardsLarge {
		count = 21
	}

	for index := int32(0); index < 21; index++ {
		award, awarded := cards.Awards[index]
		reveal, shown := cards.Revealed[index]
		kind, pending := cards.Requested[index]
		if awarded {
			awards = append(awards, map[string]any{
				"index":            index,
				"player_id":        optional(state.CardOwner(award)),
				"header_player_id": award.PlayerID,
				"living_id":        award.LivingID,
				"system_selected":  award.SystemSelected,
				"template_id":      award.TemplateID,
				"count":            award.Count,
				"vip":              optional(award.VIP),
			})
		}

		if shown {
			revealed = append(revealed, map[string]any{"index": index, "template_id": reveal.TemplateID, "count": reveal.Count})
		}

		if pending {
			requested = append(requested, map[string]any{"index": index, "kind": kind})
		}

		if index < count && !cards.Closed && !cards.Finished && !cards.SystemSelection && !awarded && !shown && !pending {
			available = append(available, index)
		}
	}

	return map[string]any{
		"known":             cards.Known,
		"revision":          cards.Revision,
		"type":              cards.Type,
		"has_next_mission":  cards.HasNextMission,
		"can_enter_final":   cards.CanEnterFinal,
		"players":           players,
		"awards":            awards,
		"revealed":          revealed,
		"requested":         requested,
		"available_indices": available,
		"finished":          cards.Finished,
		"closed":            cards.Closed,
		"system_selection":  cards.SystemSelection,
	}
}
