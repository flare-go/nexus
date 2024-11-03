package nexus

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"goflare.io/nexus/cloud"
	"goflare.io/nexus/driver"
)

// Database defines the type of database to use
type Database string

const (
	Postgres  Database = "postgres"
	Cockroach Database = "cockroach"
)

// Config defines the configuration for Nexus
type Config struct {

	// Mode defines the running mode of Nexus
	Mode Mode `yaml:"mode"`

	// Database defines the type of database to use
	Database Database `yaml:"database"`

	// Environment defines the running environment of Nexus
	Environment Environment `yaml:"environment"`

	// Postgres defines the configuration for the database
	Postgres driver.PostgresConfig `yaml:"postgres"`

	// Cockroach defines the configuration for the database
	Cockroach driver.PostgresConfig `yaml:"cockroach"`

	// Redis defines the configuration for Redis
	Redis driver.RedisConfig `yaml:"redis"`

	// NATS defines the configuration for NATS
	NATS driver.NatsConfig `yaml:"nats"`

	// Google defines the configuration for Google Cloud
	Google cloud.GoogleConfig `yaml:"google"`

	// Firebase defines the configuration for Firebase
	Firebase cloud.FirebaseConfig `yaml:"firebase"`

	// Paseto defines the configuration for Paseto
	Paseto PasetoConfig `yaml:"paseto"`

	// Stripe defines the configuration for Stripe
	Stripe StripeConfig `yaml:"stripe"`
}

// LoadConfig loads the configuration from the given path
func (c *Core) LoadConfig(path string) error {

	// Read the configuration file
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal the configuration file into the Config struct
	c.config = &Config{}
	if err := yaml.Unmarshal(data, c.config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Log the successful loading of the configuration file
	c.logger.Info("Configuration file loaded successfully")
	return nil
}
