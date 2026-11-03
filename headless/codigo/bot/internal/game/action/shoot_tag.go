package action

type ShootTag struct {
	Started  bool
	ShotType int8
}

func (ShootTag) Name() string { return "shoot_tag" }
