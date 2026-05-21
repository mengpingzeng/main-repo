package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Scheduler SchedulerConfig `yaml:"scheduler"`
	Database  DatabaseConfig  `yaml:"database"`
}

type SchedulerConfig struct {
	CronExpr      string        `yaml:"cron_expr"      env:"SCHEDULER_CRON_EXPR"`
	BatchSize     int           `yaml:"batch_size"     env:"SCHEDULER_BATCH_SIZE"`
	FetchTimeout  time.Duration `yaml:"fetch_timeout"  env:"SCHEDULER_FETCH_TIMEOUT"`
	BatchInterval time.Duration `yaml:"batch_interval" env:"SCHEDULER_BATCH_INTERVAL"`
	LookbackDays  int           `yaml:"lookback_days"  env:"SCHEDULER_LOOKBACK_DAYS"`
	MaxRetry      int           `yaml:"max_retry"      env:"SCHEDULER_MAX_RETRY"`
	RetryBackoff  time.Duration `yaml:"retry_backoff"  env:"SCHEDULER_RETRY_BACKOFF"`
	ListenPort    int           `yaml:"listen_port"    env:"SCHEDULER_LISTEN_PORT"`
}

type DatabaseConfig struct {
	DSN string `yaml:"dsn" env:"SCHEDULER_DB_DSN"`
}

func DefaultConfig() *Config {
	return &Config{
		Scheduler: SchedulerConfig{
			CronExpr:      "0,30 * * * *",
			BatchSize:     10,
			FetchTimeout:  30 * time.Second,
			BatchInterval: 200 * time.Millisecond,
			LookbackDays:  30,
			MaxRetry:      1,
			RetryBackoff:  1 * time.Second,
			ListenPort:    9104,
		},
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}
