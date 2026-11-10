package adminconfig

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoadAccessLogDefaults(t *testing.T) {
	for _, key := range []string{"ADMIN_ACCESS_LOG_QUEUE_SIZE", "ADMIN_ACCESS_LOG_BATCH_SIZE", "ADMIN_ACCESS_LOG_FLUSH_INTERVAL_MS", "ADMIN_ACCESS_LOG_RETENTION_DAYS"} {
		t.Setenv(key, "")
	}
	// Unset (not empty) selects the bundled default.
	t.Setenv("ADMIN_ACCESS_LOG_SINKS", "x")
	unsetEnv(t, "ADMIN_ACCESS_LOG_SINKS")
	c := Load()
	if strings.Join(c.AccessLogSinks, ",") != "db,stdout" {
		t.Fatalf("sinks = %v", c.AccessLogSinks)
	}
	if c.AccessLogQueueSize != 4096 || c.AccessLogBatchSize != 200 || c.AccessLogFlushInterval != time.Second || c.AccessLogRetentionDays != 30 {
		t.Fatalf("defaults = %d %d %s %d", c.AccessLogQueueSize, c.AccessLogBatchSize, c.AccessLogFlushInterval, c.AccessLogRetentionDays)
	}
	if c.AccessLogRetention() != 30*24*time.Hour {
		t.Fatalf("retention = %s", c.AccessLogRetention())
	}
}

func TestLoadAccessLogEnv(t *testing.T) {
	t.Setenv("ADMIN_ACCESS_LOG_SINKS", " Stdout, db ,stdout,, ")
	t.Setenv("ADMIN_ACCESS_LOG_QUEUE_SIZE", "100")
	t.Setenv("ADMIN_ACCESS_LOG_BATCH_SIZE", "10")
	t.Setenv("ADMIN_ACCESS_LOG_FLUSH_INTERVAL_MS", "250")
	t.Setenv("ADMIN_ACCESS_LOG_RETENTION_DAYS", "0")
	c := Load()
	if strings.Join(c.AccessLogSinks, ",") != "stdout,db" {
		t.Fatalf("sinks = %v", c.AccessLogSinks)
	}
	if c.AccessLogQueueSize != 100 || c.AccessLogBatchSize != 10 || c.AccessLogFlushInterval != 250*time.Millisecond || c.AccessLogRetention() != 0 {
		t.Fatalf("values = %+v", c)
	}
	for _, off := range []string{"", "none", " NONE "} {
		t.Setenv("ADMIN_ACCESS_LOG_SINKS", off)
		if sinks := Load().AccessLogSinks; len(sinks) != 0 {
			t.Fatalf("%q sinks = %v", off, sinks)
		}
	}
	t.Setenv("ADMIN_ACCESS_LOG_QUEUE_SIZE", "lots")
	if err := Load().Validate(); err == nil || !strings.Contains(err.Error(), "ADMIN_ACCESS_LOG_QUEUE_SIZE") {
		t.Fatalf("invalid integer = %v", err)
	}
}

func TestAccessLogValidation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"bad sink name", func(c *Config) { c.AccessLogSinks = []string{"db/x"} }, "ADMIN_ACCESS_LOG_SINKS"},
		{"negative queue", func(c *Config) { c.AccessLogQueueSize = -1 }, "ADMIN_ACCESS_LOG_QUEUE_SIZE"},
		{"huge batch", func(c *Config) { c.AccessLogBatchSize = 10_001 }, "ADMIN_ACCESS_LOG_BATCH_SIZE"},
		{"tiny flush", func(c *Config) { c.AccessLogFlushInterval = time.Millisecond }, "ADMIN_ACCESS_LOG_FLUSH_INTERVAL_MS"},
		{"negative retention", func(c *Config) { c.AccessLogRetentionDays = -1 }, "ADMIN_ACCESS_LOG_RETENTION_DAYS"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validStorageBaseConfig()
			tc.mutate(&c)
			if err := c.Validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate = %v, want %s", err, tc.want)
			}
		})
	}
	c := validStorageBaseConfig()
	c.AccessLogSinks = []string{"db", "stdout"}
	if err := c.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	clone := c.Clone()
	c.AccessLogSinks[0] = "changed"
	if clone.AccessLogSinks[0] != "db" {
		t.Fatal("Clone shares AccessLogSinks")
	}
}

// unsetEnv removes key for the rest of the test; call t.Setenv first so the
// original value is restored afterwards.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
}
