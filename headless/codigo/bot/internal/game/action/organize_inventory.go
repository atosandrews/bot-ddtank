package action

type OrganizeInventory struct {
	BagType  int32
	Places   []int32
	Revision uint64
}

func (OrganizeInventory) Name() string { return "organize_inventory" }
