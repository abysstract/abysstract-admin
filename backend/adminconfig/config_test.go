package adminconfig

import (
	"os"
	"strings"
	"testing"
	"testing/quick"
	"time"
)

// Feature: open-source-readiness, Property 12: 运行时配置范围校验
//
// 对任意配置取值：
//   - 优雅停机超时落在 1..300 秒之外 → 校验失败
//   - 最大空闲连接数 > 最大连接数 → 校验失败
//   - 所有取值在合法范围且 maxIdle ≤ maxOpen → 校验通过
//
// 用 testing/quick 随机生成 (shutdownSecs, maxOpen, maxIdle) 三元组，
// 独立参考实现 expectedRangeCheck 决定期望结果。
func TestConfigRangeCheck_Property12(t *testing.T) {
	prop := func(shutdownSecs32 int32, maxOpen32 int32, maxIdle32 int32) bool {
		shutdownSecs := int(shutdownSecs32)
		maxOpen := int(maxOpen32)
		maxIdle := int(maxIdle32)
		// 取 [0, 2*maxOpen] 让 maxIdle > maxOpen 的情况可达。
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
			t.Logf("expected error for shutdown=%d maxOpen=%d maxIdle=%d, got nil",
				shutdownSecs, maxOpen, maxIdle)
			return false
		}
		if !wantErr && err != nil {
			// 安全校验在 dev 下不阻塞（返回 nil），所以只要 rangeOK 就该过。
			t.Logf("unexpected error for shutdown=%d maxOpen=%d maxIdle=%d: %v",
				shutdownSecs, maxOpen, maxIdle, err)
			return false
		}
		return true
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 300}); err != nil {
		t.Fatalf("Property 12 failed: %v", err)
	}
}

// rangeOK 是配置范围校验的独立参考实现（只看 Property 12 关注的三项）。
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

// 显式覆盖关键边界：
func TestConfigRangeCheck_Boundaries(t *testing.T) {
	cases := []struct {
		name        string
		shutdown    int
		maxOpen     int
		maxIdle     int
		wantErrSubs string // 空串表示应通过
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

// TestLoadRejectsInvalidEnvValues 验证非法环境变量会返回错误：
// 环境变量「存在但格式非法」（非数字、负时长）必须在 Validate 时拒绝启动，
// 而不是静默回退默认值。
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
			defer os.Unsetenv(tc.key)
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

func TestCloneIsolatesSliceMutation(t *testing.T) {
	c := Config{
		Env:      EnvDev,
		DBDriver: "postgres", DBDSN: "non-default",
		JWTSecret:           "non-default",
		JWTExpiresIn:        time.Hour,
		CORSOrigins:         []string{"https://example.com"},
		ShutdownTimeout:     30 * time.Second,
		DBMaxOpenConns:      20,
		DBMaxIdleConns:      10,
		LoginMaxFailures:    5,
		LoginLockoutSeconds: 900,
		UploadMaxBytes:      1 << 20,
		AvatarMaxBytes:      1 << 20,
	}
	cloned := c.Clone()
	c.CORSOrigins[0] = "https://evil.example"
	if cloned.CORSOrigins[0] != "https://example.com" {
		t.Fatalf("clone mutated: %v", cloned.CORSOrigins)
	}
}

func TestLogValueHidesSecrets(t *testing.T) {
	c := Config{
		Env:      EnvDev,
		DBDriver: "postgres", DBDSN: "host=localhost user=admin password=super-secret dbname=admin",
		JWTSecret: "super-secret-jwt",
		Addr:      ":8080",
	}
	text := c.LogValue().String()
	if strings.Contains(text, "super-secret") || strings.Contains(text, "password=super-secret") {
		t.Fatalf("log leaked secret: %s", text)
	}
	if !strings.Contains(text, "[redacted]") {
		t.Fatalf("log missing redaction: %s", text)
	}
}

func TestValidateDatabase(t *testing.T) {
	c := Config{DBDriver: "postgres", DBDSN: "host=localhost", DBMaxOpenConns: 20, DBMaxIdleConns: 10}
	if err := c.ValidateDatabase(); err != nil {
		t.Fatal(err)
	}
	c.DBDSN = ""
	if err := c.ValidateDatabase(); err == nil || !strings.Contains(err.Error(), "ADMIN_DB_DSN") {
		t.Fatalf("got %v", err)
	}
}

func TestDatabaseEntry(t *testing.T) {
	t.Setenv("ADMIN_PG_DSN", "legacy-secret")
	t.Setenv("ADMIN_DB_DRIVER", "")
	t.Setenv("ADMIN_DB_DSN", "")
	c := Load()
	if c.DBDriver != "" || c.DBDSN != "" || c.ValidateDatabase() == nil {
		t.Fatal("explicit database required")
	}
	for _, driver := range []string{"unknown"} {
		t.Setenv("ADMIN_DB_DRIVER", driver)
		c = Load()
		if c.ValidateDatabaseDriver() == nil || c.DBDSN != "" {
			t.Fatalf("unsupported driver %s", driver)
		}
	}
	t.Setenv("ADMIN_DB_DRIVER", "postgres")
	c = Load()
	if c.ValidateDatabaseDriver() != nil || c.ValidateDatabase() == nil {
		t.Fatal("offline and online validation must differ")
	}
	t.Setenv("ADMIN_DB_DSN", "selected-secret")
	c = Load()
	if c.DBDSN != "selected-secret" || c.ValidateDatabase() != nil {
		t.Fatal("new entry")
	}
	if strings.Contains(c.LogValue().String(), "secret") && strings.Contains(c.LogValue().String(), "selected-secret") {
		t.Fatal("secret leaked")
	}
}

func TestDatabasePoolParseErrors(t *testing.T) {
	t.Setenv("ADMIN_DB_DRIVER", "postgres")
	t.Setenv("ADMIN_DB_DSN", "host=127.0.0.1 port=1 dbname=must_not_connect")
	for _, key := range []string{"ADMIN_DB_MAX_OPEN_CONNS", "ADMIN_DB_MAX_IDLE_CONNS"} {
		for _, input := range []string{"abc", "1.5", "9223372036854775808", "-9223372036854775809"} {
			t.Run(key+"/"+input, func(t *testing.T) {
				t.Setenv("ADMIN_DB_MAX_OPEN_CONNS", "20")
				t.Setenv("ADMIN_DB_MAX_IDLE_CONNS", "10")
				t.Setenv(key, input)
				c := Load()
				cloned := c.Clone()
				for _, err := range []error{c.ValidateDatabase(), c.Validate(), cloned.ValidateDatabase()} {
					if err == nil || !strings.Contains(err.Error(), key) || !strings.Contains(err.Error(), input) {
						t.Fatalf("parse error source: %v", err)
					}
				}
				if err := c.ValidateDatabaseDriver(); err != nil {
					t.Fatalf("offline driver: %v", err)
				}
			})
		}
	}
}

func TestDatabaseValidationBoundaries(t *testing.T) {
	t.Setenv("ADMIN_DB_DRIVER", "postgres")
	t.Setenv("ADMIN_DB_DSN", "host=127.0.0.1 port=1 dbname=must_not_connect")
	t.Setenv("ADMIN_ENV", "prod")
	t.Setenv("ADMIN_JWT_SECRET", "")
	t.Setenv("ADMIN_CORS_ORIGINS", "*")
	t.Setenv("ADMIN_JWT_EXPIRES_SECONDS", "invalid")
	for _, values := range [][2]string{{"", ""}, {"1", "0"}, {"1000", "1000"}} {
		t.Run("open="+values[0]+"/idle="+values[1], func(t *testing.T) {
			t.Setenv("ADMIN_DB_MAX_OPEN_CONNS", values[0])
			t.Setenv("ADMIN_DB_MAX_IDLE_CONNS", values[1])
			c := Load()
			if err := c.ValidateDatabase(); err != nil {
				t.Fatal(err)
			}
			if err := c.Validate(); err == nil {
				t.Fatal("server must enforce its application settings")
			}
			if values[0] == "" && (c.DBMaxOpenConns != 20 || c.DBMaxIdleConns != 10) {
				t.Fatal("pool defaults")
			}
		})
	}
	t.Run("application-security", func(t *testing.T) {
		t.Setenv("ADMIN_DB_MAX_OPEN_CONNS", "20")
		t.Setenv("ADMIN_DB_MAX_IDLE_CONNS", "10")
		t.Setenv("ADMIN_JWT_EXPIRES_SECONDS", "3600")
		c := Load()
		if err := c.ValidateDatabase(); err != nil {
			t.Fatal(err)
		}
		err := c.Validate()
		if err == nil || !strings.Contains(err.Error(), "ADMIN_JWT_SECRET") || !strings.Contains(err.Error(), "ADMIN_CORS_ORIGINS") {
			t.Fatalf("server security boundary: %v", err)
		}
	})

}

func TestClonePreservesIndependentDatabaseParseErrors(t *testing.T) {
	t.Setenv("ADMIN_DB_DRIVER", "postgres")
	t.Setenv("ADMIN_DB_DSN", "host=127.0.0.1 port=1 dbname=must_not_connect")
	t.Setenv("ADMIN_DB_MAX_IDLE_CONNS", "10")
	t.Setenv("ADMIN_DB_MAX_OPEN_CONNS", "abc")
	c := Load()
	cloned := c.Clone()
	c.databaseParseErrs[0] = "changed"
	if err := cloned.ValidateDatabase(); err == nil || !strings.Contains(err.Error(), "ADMIN_DB_MAX_OPEN_CONNS") || strings.Contains(err.Error(), "changed") {
		t.Fatalf("clone lost parse source: %v", err)
	}
}

func TestSQLitePoolDefaults(t *testing.T) {
	t.Setenv("ADMIN_DB_DRIVER", "sqlite")
	t.Setenv("ADMIN_DB_DSN", "file:/tmp/admin-config.sqlite")
	t.Setenv("ADMIN_DB_MAX_OPEN_CONNS", "")
	t.Setenv("ADMIN_DB_MAX_IDLE_CONNS", "")
	cfg := Load()
	if err := cfg.ValidateDatabase(); err != nil {
		t.Fatal(err)
	}
	if cfg.DBMaxOpenConns != 4 || cfg.DBMaxIdleConns != 4 {
		t.Fatalf("pool=%d/%d", cfg.DBMaxOpenConns, cfg.DBMaxIdleConns)
	}
}

func validStorageBaseConfig() Config {
	return Config{
		Env:      EnvDev,
		DBDriver: "postgres", DBDSN: "non-default",
		JWTSecret:           "non-default",
		JWTExpiresIn:        time.Hour,
		CORSOrigins:         []string{"https://example.com"},
		ShutdownTimeout:     30 * time.Second,
		DBMaxOpenConns:      20,
		DBMaxIdleConns:      10,
		LoginMaxFailures:    5,
		LoginLockoutSeconds: 900,
		UploadMaxBytes:      1 << 20,
		AvatarMaxBytes:      1 << 20,
	}
}

func TestStorageDefaults(t *testing.T) {
	c := validStorageBaseConfig()
	if err := c.Validate(); err != nil {
		t.Fatalf("zero storage config rejected: %v", err)
	}
	if c.StorageDriverOrDefault() != StorageDriverLocal {
		t.Fatalf("driver = %q", c.StorageDriverOrDefault())
	}
	if c.StorageUploadTTLOrDefault() != 15*time.Minute {
		t.Fatalf("ttl = %s", c.StorageUploadTTLOrDefault())
	}
	for _, key := range []string{"ADMIN_STORAGE_DRIVER", "ADMIN_STORAGE_UPLOAD_TTL_SECONDS"} {
		t.Setenv(key, "")
	}
	loaded := Load()
	if loaded.StorageDriver != StorageDriverLocal || loaded.StorageUploadTTL != 15*time.Minute {
		t.Fatalf("Load defaults = %q %s", loaded.StorageDriver, loaded.StorageUploadTTL)
	}
}

func TestLoadStorageEnv(t *testing.T) {
	t.Setenv("ADMIN_STORAGE_DRIVER", " S3 ")
	t.Setenv("ADMIN_STORAGE_PUBLIC_BASE_URL", "https://img.example.com/")
	t.Setenv("ADMIN_STORAGE_S3_BUCKET", "media")
	t.Setenv("ADMIN_STORAGE_S3_REGION", "auto")
	t.Setenv("ADMIN_STORAGE_S3_ENDPOINT", "http://localhost:9000")
	t.Setenv("ADMIN_STORAGE_S3_PATH_STYLE", "true")
	t.Setenv("ADMIN_STORAGE_UPLOAD_TTL_SECONDS", "120")
	t.Setenv("ADMIN_STORAGE_SIGNING_SECRET", "a-signing-secret-long-enough")
	c := Load()
	if c.StorageDriver != StorageDriverS3 || c.StorageS3Bucket != "media" || c.StorageS3Region != "auto" ||
		c.StorageS3Endpoint != "http://localhost:9000" || !c.StorageS3PathStyle || c.StorageUploadTTL != 2*time.Minute ||
		c.StoragePublicBaseURL != "https://img.example.com/" || c.StorageSigningSecret != "a-signing-secret-long-enough" {
		t.Fatalf("Load = %+v", c)
	}
	if text := c.LogValue().String(); strings.Contains(text, "a-signing-secret") {
		t.Fatalf("log leaked storage secret: %s", text)
	}
}

func TestStorageValidation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"s3 without bucket", func(c *Config) { c.StorageDriver = "s3"; c.StorageS3Region = "us-east-1" }, "ADMIN_STORAGE_S3_BUCKET"},
		{"s3 without region", func(c *Config) { c.StorageDriver = "s3"; c.StorageS3Bucket = "b" }, "ADMIN_STORAGE_S3_REGION"},
		{"s3 bad endpoint", func(c *Config) {
			c.StorageDriver, c.StorageS3Bucket, c.StorageS3Region, c.StorageS3Endpoint = "s3", "b", "r", "localhost:9000"
		}, "ADMIN_STORAGE_S3_ENDPOINT"},
		{"bad public url", func(c *Config) { c.StoragePublicBaseURL = "cdn.example.com" }, "ADMIN_STORAGE_PUBLIC_BASE_URL"},
		{"protocol-relative public url", func(c *Config) { c.StoragePublicBaseURL = "//cdn.example.com/" }, "ADMIN_STORAGE_PUBLIC_BASE_URL"},
		{"short secret", func(c *Config) { c.StorageSigningSecret = "short" }, "ADMIN_STORAGE_SIGNING_SECRET"},
		{"ttl too long", func(c *Config) { c.StorageUploadTTL = 8 * 24 * time.Hour }, "ADMIN_STORAGE_UPLOAD_TTL_SECONDS"},
		{"bad driver name", func(c *Config) { c.StorageDriver = "a/b" }, "ADMIN_STORAGE_DRIVER"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validStorageBaseConfig()
			tc.mutate(&c)
			err := c.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate = %v, want mention of %s", err, tc.want)
			}
		})
	}
	ok := validStorageBaseConfig()
	ok.StorageDriver, ok.StorageS3Bucket, ok.StorageS3Region = "s3", "media", "us-east-1"
	ok.StoragePublicBaseURL = "/files/"
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid s3 config rejected: %v", err)
	}
	t.Setenv("ADMIN_STORAGE_UPLOAD_TTL_SECONDS", "soon")
	if err := Load().Validate(); err == nil || !strings.Contains(err.Error(), "ADMIN_STORAGE_UPLOAD_TTL_SECONDS") {
		t.Fatalf("invalid ttl env = %v", err)
	}
}
