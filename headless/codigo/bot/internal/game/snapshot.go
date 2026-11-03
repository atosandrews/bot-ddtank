package game

import (
	"ddbot/bot/internal/game/captcha"
	"ddbot/bot/internal/game/fight"
	"ddbot/bot/internal/game/player"
	"ddbot/bot/internal/game/room"
	"ddbot/bot/internal/game/social"
	"ddbot/bot/internal/game/value"
)

type Snapshot struct {
	Stream   uint64
	Revision uint64
	Valid    bool
	Player   player.State
	Room     room.State
	Fight    fight.State
	Social   social.State
	Captcha  captcha.State
	Message  *Message
}

func (s Snapshot) Clone() Snapshot {
	s.Player = s.Player.Clone()
	s.Room = s.Room.Clone()
	s.Fight = s.Fight.Clone()
	s.Social = s.Social.Clone()
	s.Message = value.Copy(s.Message)

	return s
}
