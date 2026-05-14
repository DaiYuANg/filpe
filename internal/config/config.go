package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/arcgolabs/configx"
)

const defaultConfigPath = "./config.json"

type Config struct {
	HTTPAddress        string `koanf:"http_address" json:"http_address" validate:"required,min=1"`
	DataDir            string `koanf:"data_dir" json:"data_dir" validate:"required,min=1"`
	LogLevel           string `koanf:"log_level" json:"log_level" validate:"required,oneof=debug info warn error"`
	RaftNodeID         uint64 `koanf:"raft_node_id" json:"raft_node_id"`
	RaftShardID        uint64 `koanf:"raft_shard_id" json:"raft_shard_id"`
	RaftAddress        string `koanf:"raft_address" json:"raft_address"`
	RaftDataDir        string `koanf:"raft_data_dir" json:"raft_data_dir"`
	RaftBootstrap      bool   `koanf:"raft_bootstrap" json:"raft_bootstrap"`
	RaftJoin           bool   `koanf:"raft_join" json:"raft_join"`
	RaftInitialMembers string `koanf:"raft_initial_members" json:"raft_initial_members"`
}

func Default() Config {
	return Config{
		HTTPAddress:   ":8080",
		DataDir:       "./data",
		LogLevel:      "info",
		RaftNodeID:    1,
		RaftShardID:   1,
		RaftAddress:   "127.0.0.1:63000",
		RaftDataDir:   "raft",
		RaftBootstrap: true,
	}
}

func Load(opts ...configx.Option) (Config, error) {
	cfg := Default()

	options := []configx.Option{
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
	if _, statErr := os.Stat(defaultConfigPath); statErr == nil {
		options = append(options, configx.WithFiles(defaultConfigPath))
	} else if !os.IsNotExist(statErr) {
		return cfg, fmt.Errorf("check config path: %w", statErr)
	}
	options = append(options, opts...)

	loaded, err := configx.LoadTErr[Config](options...)
	if err != nil {
		return cfg, fmt.Errorf("load config failed: %w", err)
	}
	cfg = loaded

	cfg.DataDir = strings.TrimSpace(cfg.DataDir)
	if cfg.DataDir == "" {
		return cfg, errors.New("invalid config: data_dir is required")
	}
	cfg.DataDir = filepath.Clean(cfg.DataDir)
	cfg.HTTPAddress = strings.TrimSpace(cfg.HTTPAddress)
	cfg.LogLevel = strings.TrimSpace(cfg.LogLevel)
	cfg.RaftAddress = strings.TrimSpace(cfg.RaftAddress)
	cfg.RaftDataDir = strings.TrimSpace(cfg.RaftDataDir)
	cfg.RaftInitialMembers = strings.TrimSpace(cfg.RaftInitialMembers)

	if cfg.HTTPAddress == "" {
		return cfg, errors.New("invalid config: http_address is required")
	}
	if cfg.LogLevel == "" {
		return cfg, errors.New("invalid config: log_level is required")
	}
	if cfg.RaftAddress == "" {
		return cfg, errors.New("invalid config: raft_address is required")
	}
	if cfg.RaftNodeID == 0 {
		cfg.RaftNodeID = 1
	}
	if cfg.RaftShardID == 0 {
		cfg.RaftShardID = 1
	}
	if cfg.RaftDataDir == "" {
		cfg.RaftDataDir = "raft"
	}
	cfg.RaftDataDir = filepath.Clean(cfg.RaftDataDir)

	if !filepath.IsAbs(cfg.RaftDataDir) {
		cfg.RaftDataDir = filepath.Join(cfg.DataDir, cfg.RaftDataDir)
	}

	return cfg, nil
}
