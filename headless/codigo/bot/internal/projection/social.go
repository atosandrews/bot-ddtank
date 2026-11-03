package projection

import (
	"ddbot/bot/internal/game"
	"ddbot/bot/internal/game/social"
	"ddbot/bot/internal/game/value"
)

func projectSocial(next *game.Snapshot, f *fields) (string, error) {
	state := &next.Social

	switch f.integer("subcommand") {
	case 160:
		state.LastResult = &social.Result{Success: f.flag("success")}
		if !state.LastResult.Success {
			return "friend_rejected", nil
		}

		friend := social.Friend{
			ID:       f.integer("player_id"),
			Name:     f.text("nickname"),
			Relation: f.integer("relation"),
			Grade:    f.integer("grade"),
			State:    f.integer("state"),
		}
		if state.Friends == nil {
			state.Friends = make(map[int32]social.Friend)
		}
		if _, exists := state.Friends[friend.ID]; !exists && len(state.Friends) >= social.MaxFriends {
			return "", &FieldError{Name: "observed friends limit"}
		}

		state.LastResult.PlayerID = value.Of(friend.ID)
		if friend.Relation == 1 {
			delete(state.Friends, friend.ID)

			return "friend_blocked", nil
		}
		state.Friends[friend.ID] = friend
		return "friend_added", nil
	case 166:
		state.Notification = &social.Notification{Argument: f.integer("argument"), Name: f.text("nickname"), SameCity: f.flag("same_city")}
		return "friend_notification", nil
	default:
		return "", &UnsupportedProjectionError{}
	}
}

func projectInvitation(next *game.Snapshot, f *fields) (string, error) {
	invitation := social.Invitation{
		PlayerID:      f.integer("player_id"),
		RoomID:        f.integer("room_id"),
		MapID:         f.integer("map_id"),
		SecondType:    f.integer("second_type"),
		GameMode:      f.integer("game_mode"),
		Difficulty:    f.integer("difficulty"),
		LevelLimits:   f.integer("level_limits"),
		Name:          f.text("nickname"),
		RoomName:      f.text("room_name"),
		VIP:           f.flag("vip"),
		VIPLevel:      f.integer("vip_level"),
		BarrierNumber: f.integer("barrier_number"),
	}
	invitations := next.Social.Invitations

	for i, previous := range invitations {
		if previous.PlayerID == invitation.PlayerID && previous.RoomID == invitation.RoomID {
			invitations[i] = invitation

			return "room_invited", nil
		}
	}

	if len(invitations) == social.MaxInvitations {
		invitations = invitations[1:]
	}

	next.Social.Invitations = append(invitations, invitation)

	return "room_invited", nil
}
