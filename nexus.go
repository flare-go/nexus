// file: nexus/nexus.go

package nexus

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

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

	c.logger, err = zap.NewProduction()
	if err != nil {
		return fmt.Errorf("failed to New logger: %w", err)
	}

	if err = c.LoadConfig(DefaultConfigPath); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
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
		c.logger.Info(c.config.NATS.URL)
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

func ProvideConfig(c *Core) *Config {
	return c.config
}

func ProvideS3(c *Core) (*s3.S3, error) {

	sess, err := session.NewSession(&aws.Config{
		Credentials:      credentials.NewStaticCredentials(c.config.CloudFlare.AccessKey, c.config.CloudFlare.SecretKey, ""),
		Region:           aws.String("auto"),
		Endpoint:         aws.String("https://goflare.io"),
		S3ForcePathStyle: aws.Bool(true),
	})

	if err != nil {
		c.logger.Error("Failed to create session", zap.Error(err))
		return nil, err
	}

	// 创建 S3 客户端
	return s3.New(sess), nil
}

func ProvideMigration(c *Core) *migrate.Migrate {

	connStr := c.config.Postgres.URL

	if c.config.Postgres.Username != "" && c.config.Postgres.Password != "" {
		connStr += fmt.Sprintf("%s:%s@", c.config.Postgres.Username, c.config.Postgres.Password)
	}

	if c.config.Postgres.Username != "" && c.config.Postgres.Password == "" {
		connStr += fmt.Sprintf("%s@", c.config.Postgres.Username)
	}

	if c.config.Postgres.Host != "" {
		connStr += fmt.Sprintf("%s:", c.config.Postgres.Host)
	}

	if c.config.Postgres.Port != "" {
		connStr += fmt.Sprintf("%s", c.config.Postgres.Port)
	}

	if c.config.Postgres.Name != "" {
		connStr += fmt.Sprintf("/%s", c.config.Postgres.Name)
	}

	if c.config.Postgres.SSLMode != "" {
		connStr += fmt.Sprintf("?sslmode=%s", c.config.Postgres.SSLMode)
	}

	m, err := migrate.New(
		fmt.Sprintf("file://%s", c.config.Migration.Path),
		connStr,
	)
	if err != nil {
		c.logger.Error("Failed to create migration", zap.Error(err))
		return nil
	}

	return m
}
