// Package config loads and normalizes MaxIO runtime configuration.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/arcgolabs/configx"
)

const defaultConfigPath = "./config.json"

type Config struct {
	HTTPAddress            string `json:"http_address"             koanf:"http_address"             validate:"required,min=1"`
	StorageAddress         string `json:"storage_address"          koanf:"storage_address"`
	AdminToken             string `json:"admin_token"              koanf:"admin_token"`
	APIToken               string `json:"api_token"                koanf:"api_token"`
	DataDir                string `json:"data_dir"                 koanf:"data_dir"                 validate:"required,min=1"`
	LogLevel               string `json:"log_level"                koanf:"log_level"                validate:"required,oneof=debug info warn error"`
	RaftNodeID             uint64 `json:"raft_node_id"             koanf:"raft_node_id"`
	RaftShardID            uint64 `json:"raft_shard_id"            koanf:"raft_shard_id"`
	RaftAddress            string `json:"raft_address"             koanf:"raft_address"`
	RaftDataDir            string `json:"raft_data_dir"            koanf:"raft_data_dir"`
	RaftBootstrap          bool   `json:"raft_bootstrap"           koanf:"raft_bootstrap"`
	RaftJoin               bool   `json:"raft_join"                koanf:"raft_join"`
	RaftInitialMembers     string `json:"raft_initial_members"     koanf:"raft_initial_members"`
	RaftOperationTimeout   string `json:"raft_operation_timeout"   koanf:"raft_operation_timeout"   validate:"required,min=1"`
	GossipBindAddress      string `json:"gossip_bind_address"      koanf:"gossip_bind_address"      validate:"required,min=1"`
	GossipAdvertiseAddress string `json:"gossip_advertise_address" koanf:"gossip_advertise_address"`
	GossipSeeds            string `json:"gossip_seeds"             koanf:"gossip_seeds"`
	PendingObjectTTL       string `json:"pending_object_ttl"       koanf:"pending_object_ttl"       validate:"required,min=1"`
	RepairInterval         string `json:"repair_interval"          koanf:"repair_interval"          validate:"required,min=1"`
	RepairOnStart          bool   `json:"repair_on_start"          koanf:"repair_on_start"`
	RepairMaxBatch         int    `json:"repair_max_batch"         koanf:"repair_max_batch"`
	RepairMaxRetries       int    `json:"repair_max_retries"       koanf:"repair_max_retries"`
	RepairRateLimit        int    `json:"repair_rate_limit"        koanf:"repair_rate_limit"`
	RepairRetryBackoff     string `json:"repair_retry_backoff"     koanf:"repair_retry_backoff"     validate:"required,min=1"`
}

func Default() Config {
	return Config{
		HTTPAddress:          ":8080",
		StorageAddress:       "127.0.0.1:8080",
		DataDir:              "./data",
		LogLevel:             "info",
		RaftNodeID:           1,
		RaftShardID:          1,
		RaftAddress:          "127.0.0.1:63000",
		RaftDataDir:          "raft",
		RaftBootstrap:        true,
		RaftOperationTimeout: "5s",
		GossipBindAddress:    "0.0.0.0:7946",
		PendingObjectTTL:     "1h",
		RepairInterval:       "10m",
		RepairOnStart:        true,
		RepairMaxBatch:       100,
		RepairMaxRetries:     2,
		RepairRetryBackoff:   "1s",
	}
}

func Load(opts ...configx.Option) (Config, error) {
	cfg := Default()
	options, err := loadOptions(cfg, opts...)
	if err != nil {
		return cfg, err
	}

	loaded, err := configx.LoadTErr[Config](options...)
	if err != nil {
		return cfg, fmt.Errorf("load config failed: %w", err)
	}
	return normalize(loaded)
}

func loadOptions(cfg Config, opts ...configx.Option) ([]configx.Option, error) {
	options := defaultLoadOptions(cfg)
	fileOptions, err := configFileOptions(defaultConfigPath)
	if err != nil {
		return nil, err
	}
	options = append(options, fileOptions...)
	options = append(options, opts...)
	return options, nil
}

func defaultLoadOptions(cfg Config) []configx.Option {
	return []configx.Option{
		configx.WithTypedDefaults(cfg),
		configx.WithDotenv(),
		configx.WithEnvPrefix("MAXIO"),
		configx.WithEnvSeparator("__"),
		configx.WithPriority(
			configx.SourceDotenv,
			configx.SourceFile,
			configx.SourceEnv,
			configx.SourceArgs,
		),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
		configx.WithCommandLineFlags(),
		configx.WithLogger(slog.Default()),
		configx.WithWatchErrHandler(func(err error) {
			slog.Default().Error("config watch error", "error", err)
		}),
	}
}

func configFileOptions(path string) ([]configx.Option, error) {
	if _, statErr := os.Stat(path); statErr == nil {
		return []configx.Option{configx.WithFiles(path)}, nil
	} else if !os.IsNotExist(statErr) {
		return nil, fmt.Errorf("check config path: %w", statErr)
	}
	return []configx.Option{}, nil
}

func normalize(cfg Config) (Config, error) {
	cfg = trim(cfg)
	if cfg.DataDir == "" {
		return cfg, errors.New("invalid config: data_dir is required")
	}
	cfg.DataDir = filepath.Clean(cfg.DataDir)

	if err := validateRequired(cfg); err != nil {
		return cfg, err
	}
	cfg = applyZeroDefaults(cfg)
	if err := validateDurations(cfg); err != nil {
		return cfg, err
	}
	cfg.RaftDataDir = filepath.Clean(cfg.RaftDataDir)

	if !filepath.IsAbs(cfg.RaftDataDir) {
		cfg.RaftDataDir = filepath.Join(cfg.DataDir, cfg.RaftDataDir)
	}

	return cfg, nil
}

func trim(cfg Config) Config {
	cfg.DataDir = strings.TrimSpace(cfg.DataDir)
	cfg.HTTPAddress = strings.TrimSpace(cfg.HTTPAddress)
	cfg.StorageAddress = strings.TrimSpace(cfg.StorageAddress)
	cfg.AdminToken = strings.TrimSpace(cfg.AdminToken)
	cfg.APIToken = strings.TrimSpace(cfg.APIToken)
	cfg.LogLevel = strings.TrimSpace(cfg.LogLevel)
	cfg.RaftAddress = strings.TrimSpace(cfg.RaftAddress)
	cfg.RaftDataDir = strings.TrimSpace(cfg.RaftDataDir)
	cfg.RaftInitialMembers = strings.TrimSpace(cfg.RaftInitialMembers)
	cfg.RaftOperationTimeout = strings.TrimSpace(cfg.RaftOperationTimeout)
	cfg.GossipBindAddress = strings.TrimSpace(cfg.GossipBindAddress)
	cfg.GossipAdvertiseAddress = strings.TrimSpace(cfg.GossipAdvertiseAddress)
	cfg.GossipSeeds = strings.TrimSpace(cfg.GossipSeeds)
	cfg.PendingObjectTTL = strings.TrimSpace(cfg.PendingObjectTTL)
	cfg.RepairInterval = strings.TrimSpace(cfg.RepairInterval)
	cfg.RepairRetryBackoff = strings.TrimSpace(cfg.RepairRetryBackoff)
	return cfg
}

func validateRequired(cfg Config) error {
	if cfg.HTTPAddress == "" {
		return errors.New("invalid config: http_address is required")
	}
	if cfg.LogLevel == "" {
		return errors.New("invalid config: log_level is required")
	}
	if cfg.RaftAddress == "" {
		return errors.New("invalid config: raft_address is required")
	}
	if cfg.GossipBindAddress == "" {
		return errors.New("invalid config: gossip_bind_address is required")
	}
	return nil
}

func applyZeroDefaults(cfg Config) Config {
	if cfg.RaftNodeID == 0 {
		cfg.RaftNodeID = 1
	}
	if cfg.StorageAddress == "" {
		cfg.StorageAddress = storageAddressFromHTTPAddress(cfg.HTTPAddress)
	}
	if cfg.RaftShardID == 0 {
		cfg.RaftShardID = 1
	}
	if cfg.RaftDataDir == "" {
		cfg.RaftDataDir = "raft"
	}
	if cfg.PendingObjectTTL == "" {
		cfg.PendingObjectTTL = Default().PendingObjectTTL
	}
	if cfg.RaftOperationTimeout == "" {
		cfg.RaftOperationTimeout = Default().RaftOperationTimeout
	}
	if cfg.GossipBindAddress == "" {
		cfg.GossipBindAddress = Default().GossipBindAddress
	}
	return applyRepairZeroDefaults(cfg)
}

func applyRepairZeroDefaults(cfg Config) Config {
	if cfg.RepairInterval == "" {
		cfg.RepairInterval = Default().RepairInterval
	}
	if cfg.RepairRetryBackoff == "" {
		cfg.RepairRetryBackoff = Default().RepairRetryBackoff
	}
	if cfg.RepairMaxBatch <= 0 {
		cfg.RepairMaxBatch = Default().RepairMaxBatch
	}
	return cfg
}

func validateDurations(cfg Config) error {
	if _, err := time.ParseDuration(cfg.RaftOperationTimeout); err != nil {
		return fmt.Errorf("invalid config: raft_operation_timeout: %w", err)
	}
	if _, err := time.ParseDuration(cfg.PendingObjectTTL); err != nil {
		return fmt.Errorf("invalid config: pending_object_ttl: %w", err)
	}
	if _, err := time.ParseDuration(cfg.RepairInterval); err != nil {
		return fmt.Errorf("invalid config: repair_interval: %w", err)
	}
	if _, err := time.ParseDuration(cfg.RepairRetryBackoff); err != nil {
		return fmt.Errorf("invalid config: repair_retry_backoff: %w", err)
	}
	if cfg.RepairMaxRetries < 0 {
		return errors.New("invalid config: repair_max_retries must be non-negative")
	}
	if cfg.RepairRateLimit < 0 {
		return errors.New("invalid config: repair_rate_limit must be non-negative")
	}
	return nil
}

func (cfg Config) RaftOperationTimeoutDuration() time.Duration {
	duration, err := time.ParseDuration(cfg.RaftOperationTimeout)
	if err != nil {
		return 5 * time.Second
	}
	return duration
}

func (cfg Config) StorageAdvertiseAddress() string {
	if strings.TrimSpace(cfg.StorageAddress) != "" {
		return strings.TrimSpace(cfg.StorageAddress)
	}
	return storageAddressFromHTTPAddress(cfg.HTTPAddress)
}

func (cfg Config) PendingObjectTTLDuration() time.Duration {
	duration, err := time.ParseDuration(cfg.PendingObjectTTL)
	if err != nil {
		return time.Hour
	}
	return duration
}

func storageAddressFromHTTPAddress(address string) string {
	address = strings.TrimSpace(address)
	if address == "" || strings.Contains(address, "://") {
		return address
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		if strings.HasPrefix(address, ":") {
			return "127.0.0.1" + address
		}
		return address
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

func (cfg Config) RepairIntervalDuration() time.Duration {
	duration, err := time.ParseDuration(cfg.RepairInterval)
	if err != nil {
		return 10 * time.Minute
	}
	return duration
}

func (cfg Config) RepairRetryBackoffDuration() time.Duration {
	duration, err := time.ParseDuration(cfg.RepairRetryBackoff)
	if err != nil {
		return time.Second
	}
	return duration
}
