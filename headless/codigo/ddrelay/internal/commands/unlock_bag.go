package commands

type UnlockBag struct {
	Password string
}

func (UnlockBag) Name() string { return "unlock_bag" }
