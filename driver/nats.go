package driver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// NatsConfig 定義 NATS 配置
type NatsConfig struct {
	URL        string        `yaml:"url"`
	StreamName string        `yaml:"stream_name"`
	Subject    string        `yaml:"subject"`
	MaxAge     time.Duration `yaml:"max_age"`
	MaxMsgs    int64         `yaml:"max_msgs"`
	MaxBytes   int64         `yaml:"max_bytes"`
}

// NatsManager 定義 JetStream 管理器的接口
type NatsManager interface {
	Publish(ctx context.Context, subject string, data []byte) error
	Subscribe(subject string, handler nats.MsgHandler, opts ...nats.SubOpt) (*nats.Subscription, error)
	HealthCheck(ctx context.Context) error
	Close() error
}

// NatsOption 定義可選配置
type NatsOption struct {
	Logger *zap.Logger
	Config NatsConfig
}

// jetStreamNatsManager 實現 NatsManager 接口
type jetStreamNatsManager struct {
	nc     *nats.Conn
	js     nats.JetStreamContext
	logger *zap.Logger
	config NatsConfig
	mu     sync.RWMutex
}

// DefaultConfig 返回默認配置
func DefaultConfig(name, subject string) NatsConfig {
	return NatsConfig{
		StreamName: name,
		Subject:    subject,
		MaxAge:     24 * time.Hour,
		MaxMsgs:    10000,
		MaxBytes:   1024 * 1024 * 1024,
	}
}

// NewNatsManager 創建新的 JetStream 管理器
func NewNatsManager(nc *nats.Conn, opts NatsOption) (NatsManager, error) {
	if nc == nil {
		return nil, errors.New("nats connection is required")
	}

	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}

	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("failed to get jetstream context: %w", err)
	}

	mgr := &jetStreamNatsManager{
		nc:     nc,
		js:     js,
		logger: opts.Logger,
		config: opts.Config,
	}

	if err = mgr.setupStream(); err != nil {
		if errors.Is(err, nats.ErrJetStreamNotEnabled) {
			return nil, fmt.Errorf("jetstream not enabled: %w", err)
		}
		mgr.logger.Warn("stream setup issue, but continuing", zap.Error(err))
	}

	return mgr, nil
}

func (m *jetStreamNatsManager) setupStream() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	config := &nats.StreamConfig{
		Name:      m.config.StreamName,
		Subjects:  []string{m.config.Subject},
		Storage:   nats.MemoryStorage,
		Retention: nats.WorkQueuePolicy,
		MaxAge:    m.config.MaxAge,
		MaxMsgs:   m.config.MaxMsgs,
		MaxBytes:  m.config.MaxBytes,
	}

	return m.createOrUpdateStream(config)
}

func (m *jetStreamNatsManager) createOrUpdateStream(config *nats.StreamConfig) error {
	stream, err := m.js.StreamInfo(config.Name)
	if err != nil {
		if errors.Is(err, nats.ErrStreamNotFound) {
			return m.createStream(config)
		}
		return fmt.Errorf("failed to get stream info: %w", err)
	}

	return m.updateStreamIfNeeded(stream, config)
}

func (m *jetStreamNatsManager) createStream(config *nats.StreamConfig) error {
	_, err := m.js.AddStream(config)
	if err != nil {
		if strings.Contains(err.Error(), "subjects overlap") {
			m.logger.Info("using existing stream with overlapping subjects",
				zap.String("name", config.Name))
			return nil
		}
		return fmt.Errorf("failed to create stream: %w", err)
	}

	m.logger.Info("stream created successfully",
		zap.String("name", config.Name))
	return nil
}

func (m *jetStreamNatsManager) updateStreamIfNeeded(stream *nats.StreamInfo, config *nats.StreamConfig) error {
	if !m.isStreamConfigDifferent(stream.Config, *config) {
		return nil
	}

	_, err := m.js.UpdateStream(config)
	if err != nil {
		m.logger.Warn("failed to update stream config",
			zap.Error(err),
			zap.String("name", config.Name))
		return err
	}

	m.logger.Info("stream config updated successfully",
		zap.String("name", config.Name))
	return nil
}

func (m *jetStreamNatsManager) isStreamConfigDifferent(a, b nats.StreamConfig) bool {
	return a.MaxAge != b.MaxAge ||
		a.MaxMsgs != b.MaxMsgs ||
		a.MaxBytes != b.MaxBytes ||
		!stringSlicesEqual(a.Subjects, b.Subjects)
}

func (m *jetStreamNatsManager) Publish(ctx context.Context, subject string, data []byte) error {
	const (
		maxRetries = 3
		backoff    = time.Millisecond * 100
	)

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled: %w", err)
		}

		if err := m.publishWithTimeout(ctx, subject, data); err != nil {
			lastErr = err
			if attempt == maxRetries-1 {
				break
			}
			m.logRetryAttempt(subject, attempt, maxRetries, err)
			if !m.shouldRetry(err) {
				break
			}
			if err = m.sleep(ctx, backoff, attempt); err != nil {
				return err
			}
			continue
		}
		return nil
	}

	return fmt.Errorf("failed to publish after %d attempts: %w", maxRetries, lastErr)
}

func (m *jetStreamNatsManager) publishWithTimeout(ctx context.Context, subject string, data []byte) error {
	ack, err := m.js.Publish(subject, data, nats.Context(ctx))
	if err != nil {
		return err
	}

	m.logger.Info("message published",
		zap.String("subject", subject),
		zap.Uint64("sequence", ack.Sequence))
	return nil
}

func (m *jetStreamNatsManager) Subscribe(subject string, handler nats.MsgHandler, opts ...nats.SubOpt) (*nats.Subscription, error) {
	return m.js.Subscribe(subject, handler, opts...)
}

func (m *jetStreamNatsManager) getSubscriptionNatsOption(opts ...nats.SubOpt) []nats.SubOpt {
	defaultOpts := []nats.SubOpt{
		nats.ManualAck(),
		nats.AckWait(5 * time.Second),
		nats.MaxDeliver(3),
		nats.DeliverAll(),
	}
	return append(defaultOpts, opts...)
}

func (m *jetStreamNatsManager) getDurableName(subject string) string {
	return fmt.Sprintf("CHECKOUT_%s", strings.ReplaceAll(subject, ".", "_"))
}

func (m *jetStreamNatsManager) HealthCheck(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	streamInfo, err := m.js.StreamInfo(m.config.StreamName)
	if err != nil {
		return fmt.Errorf("failed to get stream info: %w", err)
	}

	m.checkMessageLimit(streamInfo)
	return nil
}

func (m *jetStreamNatsManager) Close() error {
	if m.nc != nil {
		m.nc.Close()
	}
	return nil
}

// 輔助函數
func (m *jetStreamNatsManager) checkMessageLimit(streamInfo *nats.StreamInfo) {
	usagePercentage := float64(streamInfo.State.Msgs) / float64(streamInfo.Config.MaxMsgs) * 100
	if usagePercentage >= 90 {
		m.logger.Warn("stream approaching message limit",
			zap.String("stream", m.config.StreamName),
			zap.Uint64("current_msgs", streamInfo.State.Msgs),
			zap.Int64("max_msgs", streamInfo.Config.MaxMsgs),
			zap.Float64("usage_percentage", usagePercentage))
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (m *jetStreamNatsManager) logRetryAttempt(subject string, attempt, maxRetries int, err error) {
	m.logger.Warn("failed to publish message, retrying",
		zap.Error(err),
		zap.String("subject", subject),
		zap.Int("attempt", attempt+1),
		zap.Int("maxRetries", maxRetries))
}

func (m *jetStreamNatsManager) shouldRetry(err error) bool {
	if errors.Is(err, nats.ErrJetStreamNotEnabled) ||
		errors.Is(err, nats.ErrInvalidJSAck) {
		return false
	}
	return true
}

func (m *jetStreamNatsManager) sleep(ctx context.Context, backoff time.Duration, attempt int) error {
	// 使用指數退避策略
	sleepTime := backoff * time.Duration(1<<attempt)

	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
	case <-time.After(sleepTime):
		return nil
	}
}
