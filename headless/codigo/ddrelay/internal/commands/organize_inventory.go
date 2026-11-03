package commands

type OrganizeInventory struct {
	BagType int32
	Places  []int32
}

func (OrganizeInventory) Name() string { return "organize_inventory" }
