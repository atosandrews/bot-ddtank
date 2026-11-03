package action

type Shoot struct{ X, Y, Force, Angle int32 }

func (Shoot) Name() string { return "shoot" }
