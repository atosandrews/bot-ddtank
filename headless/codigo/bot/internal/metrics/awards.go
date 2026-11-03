package metrics

import "ddbot/bot/internal/game"

func (r *combat) awards(state game.Snapshot) {
	if state.Player.ID == nil || !state.Fight.Cards.Known {
		return
	}

	for index, award := range state.Fight.Cards.Awards {
		if r.awarded[index] || award.Count < 0 {
			continue
		}

		owner := state.Fight.CardOwner(award)
		if owner == nil || *owner != *state.Player.ID {
			continue
		}

		r.awarded[index] = true
		r.state.AwardsReceived++
		r.state.RewardItemsReceived += uint64(award.Count)
		reward, exists := r.rewards[award.TemplateID]
		if !exists && len(r.rewards) >= r.rewardLimit {
			r.state.UngroupedAwards++
			r.state.UngroupedRewardItems += uint64(award.Count)
			continue
		}

		reward.TemplateID = award.TemplateID
		reward.Awards++
		reward.Quantity += uint64(award.Count)
		r.rewards[award.TemplateID] = reward
	}
}
