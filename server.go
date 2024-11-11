package nexus

type ServerConfig struct {
	Port int `yaml:"port"`
}

type Services struct {
	Auth    ServiceConfig `yaml:"auth"`
	Payment ServiceConfig `yaml:"payment"`
	Order   ServiceConfig `yaml:"order"`
	Cart    ServiceConfig `yaml:"cart"`
}

type ServiceConfig struct {
	URL string `yaml:"url"`
}
