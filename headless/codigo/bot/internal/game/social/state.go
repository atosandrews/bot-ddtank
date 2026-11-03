package social

import (
	"maps"
	"slices"

	"ddbot/bot/internal/game/value"
)

const MaxFriends = 1024
const MaxInvitations = 64

type State struct {
	Friends      map[int32]Friend
	LastResult   *Result
	Notification *Notification
	Invitations  []Invitation
}

func (s State) Clone() State {
	s.Friends = maps.Clone(s.Friends)
	s.LastResult = value.Copy(s.LastResult)
	if s.LastResult != nil {
		s.LastResult.PlayerID = value.Copy(s.LastResult.PlayerID)
	}

	s.Notification = value.Copy(s.Notification)
	s.Invitations = slices.Clone(s.Invitations)

	return s
}
