package player

type Item struct {
	BagType    int32
	Place      int32
	TemplateID int32
	Count      int32
	ValidDate  int32
	BeginDate  string
	Used       bool
}
