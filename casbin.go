package nexus

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/url"
	"os"

	pgadapter "github.com/casbin/casbin-pg-adapter"
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/go-pg/pg/v10"
	"go.uber.org/zap"
)

// ProvideEnforcer provides a new Casbin enforcer.
func ProvideEnforcer(c *Core) (*casbin.Enforcer, error) {

	m, err := model.NewModelFromFile("./configs/casbin/casbin.conf")
	if err != nil {
		c.logger.Error("無法從文件創建新模型", zap.Error(err))
		return nil, fmt.Errorf("無法從文件創建新模型: %w", err)
	}

	postgresUrl := c.config.Postgres.URL
	if postgresUrl == "" {
		c.logger.Error("無法獲取 Postgres URL")
		return nil, fmt.Errorf("無法獲取 Postgres URL")
	}

	if c.config.Postgres.Username != "" && c.config.Postgres.Password != "" {
		postgresUrl += fmt.Sprintf("%s:%s@", c.config.Postgres.Username, c.config.Postgres.Password)
	}

	if c.config.Postgres.Username != "" && c.config.Postgres.Password == "" {
		postgresUrl += fmt.Sprintf("%s@", c.config.Postgres.Username)
	}

	if c.config.Postgres.Host != "" {
		postgresUrl += fmt.Sprintf("%s:", c.config.Postgres.Host)
	}

	if c.config.Postgres.Port != "" {
		postgresUrl += fmt.Sprintf("%s", c.config.Postgres.Port)
	}

	if c.config.Postgres.Name != "" {
		postgresUrl += fmt.Sprintf("/%s", c.config.Postgres.Name)
	}

	if c.config.Postgres.SSLMode != "" {
		postgresUrl += fmt.Sprintf("?sslmode=%s", c.config.Postgres.SSLMode)
	}

	if c.config.Postgres.SSLRootCert != "" {
		postgresUrl += fmt.Sprintf("&sslrootcert=%s", c.config.Postgres.SSLRootCert)
	}

	if c.config.Postgres.Cluster != "" {
		postgresUrl += fmt.Sprintf("&options=--cluster=%s", c.config.Postgres.Cluster)
	}

	// 解析連接字符串
	opts, err := pg.ParseURL(postgresUrl)
	if err != nil {
		c.logger.Error("無法解析數據庫 URL", zap.Error(err))
		return nil, fmt.Errorf("無法解析數據庫 URL: %w", err)
	}

	// 從 URL 中提取主機名
	parsedURL, err := url.Parse(postgresUrl)
	if err != nil {
		c.logger.Error("無法解析 URL", zap.Error(err))
		return nil, fmt.Errorf("無法解析 URL: %w", err)
	}
	hostname := parsedURL.Hostname()

	// 設置 TLS 配置
	tlsConfig := &tls.Config{
		ServerName:         hostname,
		MinVersion:         tls.VersionTLS12, // 添加這行
		InsecureSkipVerify: false,            // 添加這行以確保驗證證書
	}

	switch c.config.Postgres.SSLMode {
	case "disable":
		opts.TLSConfig = nil
	case "require":
		opts.TLSConfig = tlsConfig
	case "verify-ca", "verify-full":
		rootCAs, err := x509.SystemCertPool()
		if err != nil {
			c.logger.Error("無法獲取系統證書池", zap.Error(err))
			return nil, fmt.Errorf("無法獲取系統證書池: %w", err)
		}
		if c.config.Postgres.SSLRootCert != "" {
			cert, err := os.ReadFile(c.config.Postgres.SSLRootCert)
			if err != nil {
				c.logger.Error("無法讀取 SSL 根證書", zap.Error(err))
				return nil, fmt.Errorf("無法讀取 SSL 根證書: %w", err)
			}
			if ok := rootCAs.AppendCertsFromPEM(cert); !ok {
				c.logger.Error("無法添加 SSL 根證書到證書池")
				return nil, fmt.Errorf("無法添加 SSL 根證書到證書池")
			}
		}
		tlsConfig.RootCAs = rootCAs
		opts.TLSConfig = tlsConfig
	default:
		c.logger.Error("無效的 SSL 模式", zap.String("mode", c.config.Postgres.SSLMode))
		return nil, fmt.Errorf("無效的 SSL 模式: %s", c.config.Postgres.SSLMode)
	}

	// 創建數據庫連接
	db := pg.Connect(opts)

	// 測試連接
	_, err = db.Exec("SELECT 1")
	if err != nil {
		c.logger.Error("無法連接到數據庫", zap.Error(err))
		return nil, fmt.Errorf("無法連接到數據庫: %w", err)
	}

	// 創建適配器
	adapter, err := pgadapter.NewAdapterByDB(db)
	if err != nil {
		c.logger.Error("無法創建新適配器", zap.Error(err))
		return nil, fmt.Errorf("無法創建新適配器: %w", err)
	}

	// 創建執行器
	enforcer, err := casbin.NewEnforcer(m, adapter)
	if err != nil {
		c.logger.Error("無法創建新執行器", zap.Error(err))
		return nil, fmt.Errorf("無法創建新執行器: %w", err)
	}

	return enforcer, nil
}
