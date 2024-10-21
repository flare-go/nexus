package nexus

// PasetoConfig defines the configuration for Paseto
type PasetoConfig struct {
	PublicKey  string `yaml:"public_key"`
	PrivateKey string `yaml:"private_key"`
}
