package nexus

type ServerConfig struct {
	Port            int    `yaml:"port"`
	LocalUIURL      string `yaml:"local_ui_url"`
	ProductionUIURL string `yaml:"production_ui_url"`
}

type Services struct {
	Auth    ServiceConfig `yaml:"auth"`
	Payment ServiceConfig `yaml:"payment"`
	Order   ServiceConfig `yaml:"order"`
	Cart    ServiceConfig `yaml:"cart"`
	Shop    ServiceConfig `yaml:"shop"`
}

type ServiceConfig struct {
	URL string `yaml:"url"`
}
