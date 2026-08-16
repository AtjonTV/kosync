//
// File:        internal/config/config_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package config

import (
	"testing"
	"time"
)

func TestNewUsesDocumentedDefaults(t *testing.T) {
	conf := New()

	if conf.EnableWebUi {
		t.Errorf("expected EnableWebUi to default to false")
	}
	if conf.DisableRegistration {
		t.Errorf("expected DisableRegistration to default to false")
	}
	if conf.AnalyticsRetentionDays != 90 {
		t.Errorf("expected AnalyticsRetentionDays 90, got %d", conf.AnalyticsRetentionDays)
	}
	if conf.AnalyticsRetentionMode != RetentionModeAggregate {
		t.Errorf("expected AnalyticsRetentionMode %q, got %q", RetentionModeAggregate, conf.AnalyticsRetentionMode)
	}
	if conf.AnalyticsSessionGapSeconds != 300 {
		t.Errorf("expected AnalyticsSessionGapSeconds 300, got %d", conf.AnalyticsSessionGapSeconds)
	}
	if conf.KoreaderAuthCacheTtl != 300 {
		t.Errorf("expected KoreaderAuthCacheTtl 300, got %d", conf.KoreaderAuthCacheTtl)
	}
	// The mail switches are the operator's half of a decision the account also
	// has to make, so they default to allowed rather than to silence.
	if !conf.EnableAchievementMail {
		t.Errorf("expected EnableAchievementMail to default to true")
	}
	if !conf.EnableSummaryMail {
		t.Errorf("expected EnableSummaryMail to default to true")
	}
	// A library with no limit is the default, because a limit is a decision
	// about other people's uploads that only an operator can make.
	if conf.BooksQuotaMegabytes != 0 {
		t.Errorf("expected BooksQuotaMegabytes 0, got %d", conf.BooksQuotaMegabytes)
	}
}

func TestNewReadsEnvironment(t *testing.T) {
	t.Setenv("ENABLE_WEBUI", "true")
	t.Setenv("DISABLE_REGISTRATION", "true")
	t.Setenv("ANALYTICS_RETENTION_DAYS", "14")
	t.Setenv("ANALYTICS_RETENTION_MODE", RetentionModeDelete)
	t.Setenv("KOREADER_AUTH_CACHE_TTL_SECONDS", "0")

	conf := New()

	if !conf.EnableWebUi {
		t.Errorf("expected EnableWebUi to be true")
	}
	if !conf.DisableRegistration {
		t.Errorf("expected DisableRegistration to be true")
	}
	if conf.AnalyticsRetentionDays != 14 {
		t.Errorf("expected AnalyticsRetentionDays 14, got %d", conf.AnalyticsRetentionDays)
	}
	if conf.AnalyticsRetentionMode != RetentionModeDelete {
		t.Errorf("expected AnalyticsRetentionMode %q, got %q", RetentionModeDelete, conf.AnalyticsRetentionMode)
	}
	if conf.AuthCacheTtl() != 0 {
		t.Errorf("expected caching to be disabled, got %v", conf.AuthCacheTtl())
	}
}

func TestNormalizeRejectsOutOfRangeValues(t *testing.T) {
	conf := &Config{
		AnalyticsRetentionDays:     0,
		AnalyticsRetentionMode:     "shred",
		AnalyticsWorkerInterval:    -1,
		AnalyticsWorkerBatchSize:   0,
		AnalyticsSessionGapSeconds: -5,
		AnalyticsReconcileDays:     0,
		KoreaderAuthCacheTtl:       -10,
		KoreaderAuthCacheEntries:   0,
		BooksQuotaMegabytes:        -1,
	}

	conf.Normalize()

	if conf.AnalyticsRetentionDays != 90 {
		t.Errorf("expected AnalyticsRetentionDays 90, got %d", conf.AnalyticsRetentionDays)
	}
	if conf.AnalyticsRetentionMode != RetentionModeAggregate {
		t.Errorf("expected unknown retention mode to fall back to %q, got %q", RetentionModeAggregate, conf.AnalyticsRetentionMode)
	}
	if conf.AnalyticsWorkerInterval != 5 {
		t.Errorf("expected AnalyticsWorkerInterval 5, got %d", conf.AnalyticsWorkerInterval)
	}
	if conf.AnalyticsWorkerBatchSize != 50 {
		t.Errorf("expected AnalyticsWorkerBatchSize 50, got %d", conf.AnalyticsWorkerBatchSize)
	}
	if conf.AnalyticsSessionGapSeconds != 300 {
		t.Errorf("expected AnalyticsSessionGapSeconds 300, got %d", conf.AnalyticsSessionGapSeconds)
	}
	if conf.AnalyticsReconcileDays != 7 {
		t.Errorf("expected AnalyticsReconcileDays 7, got %d", conf.AnalyticsReconcileDays)
	}
	if conf.KoreaderAuthCacheTtl != 0 {
		t.Errorf("expected negative cache ttl to clamp to 0, got %d", conf.KoreaderAuthCacheTtl)
	}
	if conf.KoreaderAuthCacheEntries != 1024 {
		t.Errorf("expected KoreaderAuthCacheEntries 1024, got %d", conf.KoreaderAuthCacheEntries)
	}
	// A negative quota is a typo, and the harmless reading of it is "no limit"
	// rather than "no uploads at all".
	if conf.BooksQuotaMegabytes != 0 {
		t.Errorf("expected a negative quota to clamp to 0, got %d", conf.BooksQuotaMegabytes)
	}
	if got := conf.QuotaBytes(); got != 0 {
		t.Errorf("expected QuotaBytes 0, got %d", got)
	}
}

func TestDurationHelpers(t *testing.T) {
	conf := &Config{
		AnalyticsWorkerInterval:    5,
		AnalyticsSessionGapSeconds: 300,
		KoreaderAuthCacheTtl:       120,
	}

	if got := conf.WorkerInterval(); got != 5*time.Second {
		t.Errorf("expected WorkerInterval 5s, got %v", got)
	}
	if got := conf.SessionGap(); got != 300*time.Second {
		t.Errorf("expected SessionGap 5m, got %v", got)
	}
	if got := conf.AuthCacheTtl(); got != 2*time.Minute {
		t.Errorf("expected AuthCacheTtl 2m, got %v", got)
	}
}
