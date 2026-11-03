package metrics

import (
	"cmp"
	"slices"

	"ddbot/bot/internal/game"
)

type combat struct {
	state       Totals
	rewardLimit int
	generation  uint64
	mapID       int32
	lastOwnTurn int32
	hasOwnTurn  bool
	awarded     map[int32]bool
	rewards     map[int32]Reward
}

func newCombat(rewardLimit int) *combat {
	return &combat{
		rewardLimit: rewardLimit,
		awarded:     make(map[int32]bool),
		rewards:     make(map[int32]Reward),
	}
}

func (r *combat) snapshot() Totals {
	snapshot := r.state
	snapshot.Rewards = make([]Reward, 0, len(r.rewards))
	for _, reward := range r.rewards {
		snapshot.Rewards = append(snapshot.Rewards, reward)
	}
	slices.SortFunc(snapshot.Rewards, func(a, b Reward) int { return cmp.Compare(a.TemplateID, b.TemplateID) })

	return snapshot
}

func (r *combat) observe(o Observation) {
	if r.generation != o.State.Fight.Generation {
		r.state.Fights.reset()
		r.resetMission()
		r.generation = o.State.Fight.Generation
	}

	switch o.Name {
	case "fight_loading":
		mapID := int32(0)
		if o.State.Fight.MapID != nil {
			mapID = *o.State.Fight.MapID
		}
		if r.state.Missions.finished || r.mapID != mapID {
			r.resetMission()
		}
		r.mapID = mapID
	case "fight_started":
		r.state.Fights.start(o.ReceivedAt)
		r.state.Missions.start(o.ReceivedAt)
	case "mission_ended":
		r.missionResult(o)
	case "ended":
		r.fightResult(o)
	case "turn_started":
		turn := o.State.Fight.Turn
		if turn != nil && ownActor(o.State, turn.ActorID) && (!r.hasOwnTurn || r.lastOwnTurn != turn.Number) {
			r.state.PlayerTurns++
			r.lastOwnTurn = turn.Number
			r.hasOwnTurn = true
		}
	case "shot_resolved":
		if ownActor(o.State, o.ActorID) {
			r.state.PlayerShotsResolved++
		}
	case "prop_used":
		if ownActor(o.State, o.ActorID) {
			r.state.PlayerPropsConfirmed++
		}
	case "card_awarded":
		r.awards(o.State)
	default:
		return
	}
}

func (r *combat) resetMission() {
	r.state.Missions.reset()
	r.mapID = 0
	r.hasOwnTurn = false
	clear(r.awarded)
}

func ownActor(state game.Snapshot, id int32) bool {
	actor, exists := state.Fight.Actors[id]
	return exists && state.Player.ID != nil && actor.PlayerID != nil && *actor.PlayerID == *state.Player.ID
}
