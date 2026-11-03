package metrics

func (r *combat) missionResult(o Observation) {
	if o.State.Player.ID == nil {
		r.state.Missions.finish(o.ReceivedAt, nil)
		return
	}

	player, exists := o.State.Fight.Cards.Players[*o.State.Player.ID]
	if !exists {
		r.state.Missions.finish(o.ReceivedAt, nil)
		return
	}

	if r.state.Missions.finish(o.ReceivedAt, &player.Winner) && player.GainedGP > 0 {
		r.state.MissionExperience += uint64(player.GainedGP)
	}
}

func (r *combat) fightResult(o Observation) {
	r.state.Missions.finish(o.ReceivedAt, nil)
	if o.UnreadableFightResult {
		r.state.UnreadableFightResults++
	}

	for _, player := range o.FightResults {
		if o.State.Player.ID == nil || player.PlayerID != *o.State.Player.ID {
			continue
		}

		if r.state.Fights.finish(o.ReceivedAt, &player.Winner) && player.GainedGP > 0 {
			r.state.FightExperience += uint64(player.GainedGP)
		}
		return
	}

	r.state.Fights.finish(o.ReceivedAt, nil)
}
