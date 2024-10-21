// file: nexus/nexus.go

package nexus

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"github.com/stripe/stripe-go/v80"
	"github.com/stripe/stripe-go/v80/client"
	"go.uber.org/zap"

	"goflare.io/nexus/driver"
)

type Mode string

type Environment string

const (
	ModeLocal Mode = "local"
	ModeCloud Mode = "cloud"

	EnvDevelopment Environment = "development"
	EnvStaging     Environment = "staging"
	EnvProduction  Environment = "production"

	DefaultConfigPath = "./configs/config.yaml"
)

// Core is the main interface for Nexus, providing various services and functionalities
type Core interface {

	// New initializes all components of Nexus
	New(ctx context.Context) error

	// LoadConfig loads the configuration from the given path
	LoadConfig(path string) error

	// Mode returns the running mode of Nexus
	Mode() Mode

	// Environment returns the running environment of Nexus
	Environment() Environment

	// DB returns the database connection pool
	DB() *driver.DB

	// NATSConn returns the NATS connection
	NATSConn() *nats.Conn

	// StripeClient returns the Stripe client
	StripeClient() *client.API

	// Logger returns the logger
	Logger() *zap.Logger

	// Config returns the configuration
	Config() *Config

	// Shutdown gracefully shuts down all components
	Shutdown() error
}

type core struct {
	config       *Config
	db           *driver.DB
	redisClient  *redis.Client
	natsConn     *nats.Conn
	stripeClient *client.API
	logger       *zap.Logger
}

func NewCore() Core {
	return &core{}
}

func (c *core) New(ctx context.Context) error {

	var err error

	c.logger, err = zap.NewProduction()
	if err != nil {
		return fmt.Errorf("failed to New logger: %w", err)
	}

	switch c.config.Database {
	case Postgres:
		c.logger.Info("Using Postgres database")
		c.db, err = driver.ConnectSQL(c.config.Postgres)
	case Cockroach:
		c.logger.Info("Using Cockroach database")
		c.db, err = driver.ConnectSQL(c.config.Cockroach)
	}

	if c.config.Redis.Address != "" {
		c.logger.Info("Using Redis")
		c.redisClient = redis.NewClient(&redis.Options{
			Addr:     c.config.Redis.Address,
			Password: c.config.Redis.Password,
			DB:       c.config.Redis.DB,
		})
		if err = c.redisClient.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("failed to connect to Redis: %w", err)
		}
	}

	if c.config.NATS.URL != "" {
		c.logger.Info("Using NATS")
		c.natsConn, err = nats.Connect(c.config.NATS.URL)
		if err != nil {
			return fmt.Errorf("failed to connect to NATS: %w", err)
		}
	}

	if c.config.Stripe.SecretKey != "" {
		c.logger.Info("Using Stripe")
		stripe.Key = c.config.Stripe.SecretKey
		c.stripeClient = client.New(c.config.Stripe.SecretKey, nil)
	}

	c.logger.Info("All components Newd successfully")
	return nil
}

func (c *core) Shutdown() error {
	c.logger.Info("Starting shutdown of all components")

	if c.db != nil {
		c.db.Pool.Close()
	}

	if c.redisClient != nil {
		if err := c.redisClient.Close(); err != nil {
			c.logger.Error("Failed to close Redis connection", zap.Error(err))
		}
	}

	if c.natsConn != nil {
		c.natsConn.Close()
	}

	c.logger.Info("All components shut down")
	return nil
}

func (c *core) Mode() Mode {
	return c.config.Mode
}

func (c *core) Environment() Environment {
	return c.config.Environment
}

func (c *core) DB() *driver.DB {
	return c.db
}

func (c *core) Redis() *redis.Client {
	return c.redisClient
}

func (c *core) NATSConn() *nats.Conn {
	return c.natsConn
}

func (c *core) StripeClient() *client.API {
	return c.stripeClient
}

func (c *core) Logger() *zap.Logger {
	return c.logger
}

func (c *core) Config() *Config {
	return c.config
}
