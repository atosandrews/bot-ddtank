package projection

import (
	"ddbot/bot/internal/game"
	"ddbot/bot/internal/game/fight"
	"ddbot/bot/internal/game/room"
	"ddbot/bot/internal/game/value"
)

func projectRoom(next *game.Snapshot, header Header, f *fields) (string, error) {
	switch f.integer("subcommand") {
	case 0:
		next.Room = room.NewState()
		next.Room.ID = value.Of(f.integer("room_id"))
		next.Room.MapID = value.Of(f.integer("map_id"))
		next.Room.Name = value.Of(f.text("name"))
		next.Room.Difficulty = value.Of(f.integer("difficulty"))
		next.Room.Started = value.Of(f.flag("started"))
		return "room_entered", nil
	case 1:
		next.Room.LastJoinSuccess = value.Of(f.flag("success"))
		if !*next.Room.LastJoinSuccess {
			return "room_join_rejected", nil
		}
		return "room_join_accepted", nil
	case 2:
		next.Room.MapID = value.Of(f.integer("map_id"))
		next.Room.Name = value.Of(f.text("name"))
		next.Room.Difficulty = value.Of(f.integer("difficulty"))
		return "room_changed", nil
	case 4:
		id := header.ClientID
		if _, exists := f.data["player_id"]; exists {
			id = f.integer("player_id")
		}
		player := room.Player{ID: id, Team: value.Of(f.integer("team")), Place: value.Of(f.integer("place"))}
		if _, exists := f.data["nickname"]; exists {
			player.Name = value.Of(f.text("nickname"))
		}
		next.Room.Players[id] = player
		return "room_player_joined", nil
	case 5:
		delete(next.Room.Players, header.ClientID)
		if next.Player.ID != nil && *next.Player.ID == header.ClientID {
			next.Room = room.NewState()
			generation := next.Fight.Generation + 1
			next.Fight = fight.NewState()
			next.Fight.Generation = generation
		}
		return "room_player_left", nil
	case 11:
		next.Room.StartCancelledRevision++
		next.Room.Started = value.Of(false)

		return "room_start_cancelled", nil
	case 6:
		player := next.Room.Players[header.ClientID]
		player.ID = header.ClientID
		player.Team = value.Of(f.integer("team"))
		player.Place = value.Of(f.integer("place"))
		next.Room.Players[player.ID] = player
		return "room_player_changed", nil
	default:
		return "", &UnsupportedProjectionError{}
	}
}
