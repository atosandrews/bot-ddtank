package headless

type Character struct {
	ID        int32  `yaml:"id"`
	WebsiteID int32  `yaml:"website_id"`
	Name      string `yaml:"name"`
	Upstream  string `yaml:"upstream"`
}
