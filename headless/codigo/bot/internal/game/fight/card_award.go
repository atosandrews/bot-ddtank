package fight

import "ddbot/bot/internal/game/value"

type CardAward struct {
	Index          int32
	PlayerID       int32
	LivingID       int32
	SystemSelected bool
	TemplateID     int32
	Count          int32
	VIP            *bool
}

func (c CardAward) Clone() CardAward {
	c.VIP = value.Copy(c.VIP)

	return c
}
