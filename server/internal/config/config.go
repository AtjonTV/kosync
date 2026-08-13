//
// File:        internal/config/config.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package config

import (
	"time"

	"git.obth.eu/atjontv/kosync/pkg/decode"
	"github.com/joho/godotenv"
)

// ConfigFileName is the optional environment file loaded from the working directory.
// Values already present in the environment take precedence over the file.
const ConfigFileName = "./kosync.env"

// Retention modes for daily reading statistics.
const (
	RetentionModeAggregate = "aggregate"
	RetentionModeDelete    = "delete"
)

// Config holds the KOsync specific settings.
//
// Everything PocketBase already owns (listen address, data dir, SMTP, backups,
// rate limits, token lifetimes) is intentionally not duplicated here and is
// configured through the PocketBase flags and the superuser settings instead.
type Config struct {
	EnableWebUi         bool `env:"ENABLE_WEBUI" default:"false"`
	DisableRegistration bool `env:"DISABLE_REGISTRATION" default:"false"`

	AnalyticsRetentionDays     int    `env:"ANALYTICS_RETENTION_DAYS" default:"90"`
	AnalyticsRetentionMode     string `env:"ANALYTICS_RETENTION_MODE" default:"aggregate"`
	AnalyticsWorkerInterval    int    `env:"ANALYTICS_WORKER_INTERVAL_SECONDS" default:"5"`
	AnalyticsWorkerBatchSize   int    `env:"ANALYTICS_WORKER_BATCH_SIZE" default:"50"`
	AnalyticsSessionGapSeconds int    `env:"ANALYTICS_SESSION_GAP_SECONDS" default:"300"`
	AnalyticsReconcileDays     int    `env:"ANALYTICS_RECONCILE_DAYS" default:"7"`

	KoreaderAuthCacheTtl     int `env:"KOREADER_AUTH_CACHE_TTL_SECONDS" default:"300"`
	KoreaderAuthCacheEntries int `env:"KOREADER_AUTH_CACHE_ENTRIES" default:"1024"`
}

// New loads the configuration from "./kosync.env" (if present) and the environment.
//
// Invalid or out-of-range values fall back to their defaults instead of failing
// the boot, so a typo in one variable cannot take a running server down.
func New() *Config {
	_ = godotenv.Load(ConfigFileName)

	conf := &Config{}
	if err := decode.StructFromEnv(conf); err != nil {
		// A decode error means no value was applied at all, so start from the
		// documented defaults rather than from a half-filled struct.
		conf = &Config{}
	}
	conf.Normalize()

	return conf
}

// Normalize replaces values that are out of range with their defaults.
func (c *Config) Normalize() {
	if c.AnalyticsRetentionDays < 1 {
		c.AnalyticsRetentionDays = 90
	}
	if c.AnalyticsRetentionMode != RetentionModeAggregate && c.AnalyticsRetentionMode != RetentionModeDelete {
		c.AnalyticsRetentionMode = RetentionModeAggregate
	}
	if c.AnalyticsWorkerInterval < 1 {
		c.AnalyticsWorkerInterval = 5
	}
	if c.AnalyticsWorkerBatchSize < 1 {
		c.AnalyticsWorkerBatchSize = 50
	}
	if c.AnalyticsSessionGapSeconds < 1 {
		c.AnalyticsSessionGapSeconds = 300
	}
	if c.AnalyticsReconcileDays < 1 {
		c.AnalyticsReconcileDays = 7
	}
	if c.KoreaderAuthCacheTtl < 0 {
		c.KoreaderAuthCacheTtl = 0
	}
	if c.KoreaderAuthCacheEntries < 1 {
		c.KoreaderAuthCacheEntries = 1024
	}
}

// WorkerInterval returns the analytics queue drain interval.
func (c *Config) WorkerInterval() time.Duration {
	return time.Duration(c.AnalyticsWorkerInterval) * time.Second
}

// SessionGap returns the maximum gap between two progress updates that is still
// counted as continuous reading time.
func (c *Config) SessionGap() time.Duration {
	return time.Duration(c.AnalyticsSessionGapSeconds) * time.Second
}

// AuthCacheTtl returns the lifetime of a verified KOReader credential in the
// in-memory cache. A zero duration disables caching.
func (c *Config) AuthCacheTtl() time.Duration {
	return time.Duration(c.KoreaderAuthCacheTtl) * time.Second
}
