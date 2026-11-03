package commands

type UseProp struct {
	BagType           int32
	Place, TemplateID int32
}

func (UseProp) Name() string { return "use_prop" }
