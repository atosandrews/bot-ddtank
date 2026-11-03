package packets

type Result struct {
	Name        string
	Status      string
	Fields      map[string]any
	Consumed    int
	Total       int
	Remaining   []byte
	Compression string
	Err         error
}
