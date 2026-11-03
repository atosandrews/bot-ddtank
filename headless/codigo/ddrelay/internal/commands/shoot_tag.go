package commands

type ShootTag struct {
	Started  bool
	ShotType int32
}

func (ShootTag) Name() string { return "shoot_tag" }
