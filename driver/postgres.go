package driver

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresPool is an interface that represents a connection pool to a driver.
type PostgresPool interface {
	// Acquire returns a connection from the pool.
	Acquire(ctx context.Context) (*pgxpool.Conn, error)

	// BeginTx starts a new transaction and returns a Tx.
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)

	// Exec executes an SQL command and returns the command tag.
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)

	// Query executes an SQL query and returns the resulting rows.
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)

	// QueryRow executes an SQL query and returns a single row.
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row

	// SendBatch sends a batch of queries to the server. The batch is executed as a single transaction.
	SendBatch(ctx context.Context, batch *pgx.Batch) pgx.BatchResults

	// Close closes the pool and all its connections.
	Close()
}

type PostgresTx interface {
	// Begin starts a pseudo nested transaction.
	Begin(ctx context.Context) (pgx.Tx, error)

	// Commit commits the transaction if this is a real transaction or releases the savepoint if this is a pseudo nested
	// transaction. Commit will return an errors where errors.Is(ErrTxClosed) is true if the Tx is already closed, but is
	// otherwise safe to call multiple times. If the commit fails with a rollback status (e.g., the transaction was already
	// in a broken state), then an errors where errors.Is(ErrTxCommitRollback) is true will be returned.
	Commit(ctx context.Context) error

	// Rollback rolls back the transaction if this is a real transaction or rolls back to the savepoint if this is a
	// pseudo nested transaction. Rollback will return an errors where errors.Is(ErrTxClosed) is true if the Tx is already
	// closed, but is otherwise safe to call multiple times. Hence, a defer tx.Rollback() is safe even if tx.Commit() will
	// be called first in a non-errors condition. Any other failure of a real transaction will result in the connection
	// being closed.
	Rollback(ctx context.Context) error

	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
	LargeObjects() pgx.LargeObjects

	Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error)

	Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row

	// Conn returns the underlying *Conn that on which this transaction is executing.
	Conn() *pgx.Conn
}

type PostgresConfig struct {
	URL         string `yaml:"url"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	Host        string `yaml:"host"`
	Port        string `yaml:"port"`
	Name        string `yaml:"name"`
	SSLMode     string `yaml:"ssl_mode"`
	SSLRootCert string `yaml:"ssl_root_cert"`
	Cluster     string `yaml:"cluster"`
}

type DB struct {
	Pool PostgresPool
}

var dbConn = &DB{}

const maxOpenDbConn = 10

const maxDbLifetime = 5 * time.Minute

func ConnectSQL(config PostgresConfig) (*DB, error) {
	connStr := config.URL

	if config.Username != "" && config.Password != "" {
		connStr += fmt.Sprintf("%s:%s@", config.Username, config.Password)
	}

	if config.Username != "" && config.Password == "" {
		connStr += fmt.Sprintf("%s@", config.Password)
	}

	if config.Host != "" {
		connStr += fmt.Sprintf("%s:", config.Host)
	}

	if config.Port != "" {
		connStr += fmt.Sprintf("%s", config.Port)
	}

	if config.Name != "" {
		connStr += fmt.Sprintf("/%s", config.Name)
	}

	if config.SSLMode != "" {
		connStr += fmt.Sprintf("?sslmode=%s", config.SSLMode)
	}

	if config.SSLRootCert != "" {
		connStr += fmt.Sprintf("&sslrootcert=%s", config.SSLRootCert)
	}

	if config.Cluster != "" {
		connStr += fmt.Sprintf("&options=--cluster=%s", config.Cluster)
	}

	pgConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("解析連接字符串失敗 | failed to parse connection string: %w", err)
	}

	pgConfig.MaxConns = int32(maxOpenDbConn)
	pgConfig.MaxConnLifetime = maxDbLifetime

	pool, err := pgxpool.NewWithConfig(context.Background(), pgConfig)
	if err != nil {
		return nil, fmt.Errorf("創建連接池失敗 | failed to create connection pool: %w", err)
	}

	dbConn.Pool = pool

	if err = testDB(pool); err != nil {
		return nil, fmt.Errorf("測試數據庫連接失敗 | failed to test database connection: %w", err)
	}

	return dbConn, nil
}

func testDB(p *pgxpool.Pool) error {
	conn, err := p.Acquire(context.Background())
	if err != nil {
		return fmt.Errorf("獲取連接失敗 | failed to acquire connection: %w", err)
	}
	defer conn.Release()
	return nil
}
