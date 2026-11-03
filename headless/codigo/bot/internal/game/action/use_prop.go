package action

type UseProp struct {
	BagType           int8
	Place, TemplateID int32
}

func (UseProp) Name() string { return "use_prop" }
