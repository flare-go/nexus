// file: nexus/nexus.go

package nexus

import (
	"context"
	"fmt"

	"cloud.google.com/go/storage"

	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"

	"github.com/stripe/stripe-go/v80"
	"github.com/stripe/stripe-go/v80/client"

	"go.uber.org/zap"

	"google.golang.org/api/option"

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

// Core is the implementation of the Core interface
type Core struct {

	// config is the configuration for Nexus
	config *Config

	// db is the database connection pool
	db *driver.DB

	// redisClient is the Redis client
	redisClient *redis.Client

	// natsConn is the NATS connection
	natsConn *nats.Conn

	// stripeClient is the Stripe client
	stripeClient *client.API

	// storageClient is the Google Cloud Storage client
	storageClient *storage.Client

	// storageBucket is the Google Cloud Storage bucket
	storageBucket *storage.BucketHandle

	// logger is the logger
	logger *zap.Logger
}

func NewCore() *Core {
	c := new(Core)

	if err := c.New(); err != nil {
		panic(err)
	}
	return c
}

func (c *Core) New() error {

	var err error

	if err = c.LoadConfig(DefaultConfigPath); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

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
		if err = c.redisClient.Ping(context.Background()).Err(); err != nil {
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

	if c.config.Google.ServiceAccountKeyPath != "" {
		client, err := storage.NewClient(context.Background(), option.WithCredentialsFile(c.config.Google.ServiceAccountKeyPath))
		if err != nil {
			return nil
		}
		c.storageClient = client

		if c.config.Google.StorageBucket != "" {
			c.storageBucket = c.storageClient.Bucket(c.config.Google.StorageBucket)
		}
	}

	c.logger.Info("All components Newd successfully")
	return nil
}

func (c *Core) Shutdown() error {
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

func ProvideMode(c *Core) Mode {
	return c.config.Mode
}

func ProvideEnvironment(c *Core) Environment {
	return c.config.Environment
}

func ProvidePostgresPool(c *Core) driver.PostgresPool {
	return c.db.Pool
}

func ProvideRedis(c *Core) *redis.Client {
	return c.redisClient
}

func ProvideNATSConn(c *Core) *nats.Conn {
	return c.natsConn
}

func ProvideStripeClient(c *Core) *client.API {
	return c.stripeClient
}

func ProvideLogger(c *Core) *zap.Logger {
	return c.logger
}

func ProvideStorageClient(c *Core) *storage.Client {
	return c.storageClient
}

func ProvideStorageBucket(c *Core) *storage.BucketHandle {
	return c.storageBucket
}

func ProvideConfig(c *Core) *Config {
	return c.config
}
