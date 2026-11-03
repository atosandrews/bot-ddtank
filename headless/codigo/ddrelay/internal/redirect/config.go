package redirect

type Config struct {
	UID   uint32   `json:"uid"`
	Port  uint16   `json:"port"`
	Ports []uint16 `json:"ports"`
	Mark  uint32   `json:"mark"`
}

func (c Config) Validate() error {
	if c.Port == 0 || c.Mark == 0 || len(c.Ports) == 0 || len(c.Ports) > 16 {
		return &Error{Operation: "config", Reason: "a listener port, mark and 1 to 16 destination ports are required"}
	}

	for _, port := range c.Ports {
		if port == 0 || port == c.Port {
			return &Error{Operation: "config", Reason: "destination port must be nonzero and differ from listener"}
		}
	}

	return nil
}
