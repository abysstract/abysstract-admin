package conf

import (
	"os"
	"strings"
	"testing"
	"testing/quick"
	"time"
)

const (
	minShutdownTimeoutSecs = 1
	maxShutdownTimeoutSecs = 300
	minDBMaxOpenConns      = 1
	maxDBMaxOpenConns      = 1000
)

func TestConfigAliasLoadAndValidate(t *testing.T) {
	t.Setenv("ADMIN_ENV", "dev")
	t.Setenv("ADMIN_JWT_SECRET", "alias-secret")
	t.Setenv("ADMIN_DB_DRIVER", "postgres")
	t.Setenv("ADMIN_DB_DSN", "host=localhost user=test password=test dbname=test sslmode=disable")
	t.Setenv("ADMIN_CORS_ORIGINS", "http://localhost:5173")
	t.Setenv("ADMIN_SHUTDOWN_TIMEOUT_SECONDS", "30")
	t.Setenv("ADMIN_DB_MAX_OPEN_CONNS", "20")
	t.Setenv("ADMIN_DB_MAX_IDLE_CONNS", "10")

	c := Load()
	if c.Env != EnvDev {
		t.Fatalf("env = %q", c.Env)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if err := c.ValidateDatabase(); err != nil {
		t.Fatalf("ValidateDatabase: %v", err)
	}
}

func TestConfigRangeCheck_Property12(t *testing.T) {
	prop := func(shutdownSecs32 int32, maxOpen32 int32, maxIdle32 int32) bool {
		shutdownSecs := int(shutdownSecs32)
		maxOpen := int(maxOpen32)
		maxIdle := int(maxIdle32)
		if maxOpen < 0 {
			maxOpen = -maxOpen
		}
		if maxOpen > 2000 {
			maxOpen = maxOpen % 2000
		}
		if maxIdle < 0 {
			maxIdle = -maxIdle
		}
		if maxIdle > 4000 {
			maxIdle = maxIdle % 4000
		}

		c := &Config{
			Env:      EnvDev,
			DBDriver: "postgres", DBDSN: "non-default-dsn",
			JWTSecret:           "non-default-secret",
			JWTExpiresIn:        time.Hour,
			CORSOrigins:         []string{"https://example.com"},
			ShutdownTimeout:     time.Duration(shutdownSecs) * time.Second,
			DBMaxOpenConns:      maxOpen,
			DBMaxIdleConns:      maxIdle,
			LoginMaxFailures:    5,
			LoginLockoutSeconds: 900,
			UploadMaxBytes:      1 << 20,
			AvatarMaxBytes:      1 << 20,
		}

		err := c.Validate()
		wantErr := !rangeOK(shutdownSecs, maxOpen, maxIdle)
		if wantErr && err == nil {
			return false
		}
		if !wantErr && err != nil {
			return false
		}
		return true
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 300}); err != nil {
		t.Fatalf("Property 12 failed: %v", err)
	}
}

func rangeOK(shutdownSecs, maxOpen, maxIdle int) bool {
	if shutdownSecs < minShutdownTimeoutSecs || shutdownSecs > maxShutdownTimeoutSecs {
		return false
	}
	if maxOpen < minDBMaxOpenConns || maxOpen > maxDBMaxOpenConns {
		return false
	}
	if maxIdle < 0 || maxIdle > maxOpen {
		return false
	}
	return true
}

func TestConfigRangeCheck_Boundaries(t *testing.T) {
	cases := []struct {
		name        string
		shutdown    int
		maxOpen     int
		maxIdle     int
		wantErrSubs string
	}{
		{"shutdown=0 拒绝", 0, 20, 10, "ADMIN_SHUTDOWN_TIMEOUT_SECONDS"},
		{"shutdown=1 边界通过", 1, 20, 10, ""},
		{"shutdown=300 边界通过", 300, 20, 10, ""},
		{"shutdown=301 拒绝", 301, 20, 10, "ADMIN_SHUTDOWN_TIMEOUT_SECONDS"},
		{"maxOpen=0 拒绝", 30, 0, 0, "ADMIN_DB_MAX_OPEN_CONNS"},
		{"maxOpen=1001 拒绝", 30, 1001, 10, "ADMIN_DB_MAX_OPEN_CONNS"},
		{"maxIdle > maxOpen 拒绝", 30, 10, 11, "ADMIN_DB_MAX_IDLE_CONNS"},
		{"maxIdle == maxOpen 通过", 30, 10, 10, ""},
		{"maxIdle = 0 通过", 30, 10, 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{
				Env:      EnvDev,
				DBDriver: "postgres", DBDSN: "non-default",
				JWTSecret:           "non-default",
				JWTExpiresIn:        time.Hour,
				CORSOrigins:         []string{"https://example.com"},
				ShutdownTimeout:     time.Duration(tc.shutdown) * time.Second,
				DBMaxOpenConns:      tc.maxOpen,
				DBMaxIdleConns:      tc.maxIdle,
				LoginMaxFailures:    5,
				LoginLockoutSeconds: 900,
				UploadMaxBytes:      1 << 20,
				AvatarMaxBytes:      1 << 20,
			}
			err := c.Validate()
			if tc.wantErrSubs == "" {
				if err != nil {
					t.Errorf("expected pass, got error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErrSubs)
			}
			if !strings.Contains(err.Error(), tc.wantErrSubs) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrSubs)
			}
		})
	}
}

func TestLoadRejectsInvalidEnvValues(t *testing.T) {
	cases := []struct {
		name  string
		key   string
		value string
	}{
		{"shutdown 非数字", "ADMIN_SHUTDOWN_TIMEOUT_SECONDS", "abc"},
		{"shutdown 为 0", "ADMIN_SHUTDOWN_TIMEOUT_SECONDS", "0"},
		{"shutdown 为负", "ADMIN_SHUTDOWN_TIMEOUT_SECONDS", "-5"},
		{"db max open 非数字", "ADMIN_DB_MAX_OPEN_CONNS", "xyz"},
		{"upload bytes 非数字", "ADMIN_UPLOAD_MAX_BYTES", "NaN"},
		{"jwt expires 非数字", "ADMIN_JWT_EXPIRES_SECONDS", "later"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			os.Unsetenv(tc.key)
			t.Cleanup(func() { os.Unsetenv(tc.key) })
			os.Setenv(tc.key, tc.value)

			c := Load()
			err := c.Validate()
			if err == nil {
				t.Fatalf("Validate 应拒绝非法 %s=%q", tc.key, tc.value)
			}
			if !strings.Contains(err.Error(), tc.key) {
				t.Errorf("错误信息应包含 %s, got: %v", tc.key, err)
			}
		})
	}
}
