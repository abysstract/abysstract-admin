// Package adminconfig is the public application configuration.
// It does not import adminapp, system, or service packages.
package adminconfig

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// 默认值常量：生产模式下若命中这些默认值会拒绝启动。
const (
	defaultJWTSecret       = "change-me-in-production"
	developmentPostgresDSN = "host=localhost port=5433 user=admin password=admin dbname=abysstract_admin sslmode=disable"
)

// 范围约束（与 design.md「配置模型」一致）。
const (
	minShutdownTimeoutSecs = 1
	maxShutdownTimeoutSecs = 300

	minDBMaxOpenConns = 1
	maxDBMaxOpenConns = 1000

	defaultShutdownTimeout = 30 * time.Second
	defaultDBMaxOpenConns  = 20
	defaultDBMaxIdleConns  = 10
	defaultJWTExpiresIn    = 1 * time.Hour
	// 刷新令牌默认 7 天；仅在 ADMIN_REFRESH_TOKEN_ENABLED=true 时生效。
	defaultJWTRefreshExpiresIn = 7 * 24 * time.Hour

	defaultLoginMaxFailures    = 5
	defaultLoginLockoutSeconds = 900

	defaultUploadMaxBytes = 50 * 1024 * 1024 // 50 MB
	defaultAvatarMaxBytes = 5 * 1024 * 1024  // 5 MB

	defaultStorageDriver    = StorageDriverLocal
	defaultStorageUploadTTL = 15 * time.Minute
	maxStorageUploadTTL     = 7 * 24 * time.Hour
	minStorageSecretLength  = 16
)

// 内置存储驱动名。其他驱动可由应用通过 storage.Register 注册后按名称选择。
const (
	StorageDriverLocal = "local"
	StorageDriverS3    = "s3"
)

// 运行模式。
const (
	EnvDev  = "dev"
	EnvProd = "prod"
)

// Config is the backend runtime configuration loaded from the environment.
type Config struct {
	Env          string // 运行模式：dev | prod
	Addr         string // 监听地址
	DBDriver     string // 数据库产品
	DBDSN        string // 数据库连接串
	JWTSecret    string // JWT 签名密钥
	JWTExpiresIn time.Duration
	CORSOrigins  []string // 允许的跨域来源（dev 默认本地，prod 必须显式配置）
	TrustProxy   bool     // 是否信任反向代理的 X-Forwarded-For（取客户端真实 IP）
	// MetricsToken 保护 /metrics：设置后需携带 Bearer token；未设置时开发模式开放、生产模式关闭。
	MetricsToken string

	// 刷新令牌（需求 6.10/6.11；默认关闭，显式启用时 LoginResponse 才携带 refresh_token）
	RefreshTokenEnabled bool          // 是否启用刷新令牌特性
	JWTRefreshExpiresIn time.Duration // 刷新令牌有效期（> JWTExpiresIn 才有意义）

	// 运行时（需求 8.10–8.12）
	ShutdownTimeout time.Duration // 优雅停机超时（1–300s）
	DBMaxOpenConns  int           // 数据库连接池最大连接数（1–1000）
	DBMaxIdleConns  int           // 数据库连接池最大空闲连接数（0..MaxOpen）

	// 登录安全（需求 6.6/6.7；具体策略在 repo/loginsec.go 实现）
	LoginMaxFailures    int // 连续失败锁定阈值（≥1）
	LoginLockoutSeconds int // 锁定时长（≥1s）

	// 上传限制（需求 9.1–9.4）
	UploadMaxBytes int64  // 通用上传大小上限
	AvatarMaxBytes int64  // 头像上传大小上限
	UploadDir      string // 本地存储驱动的根目录

	// 对象存储（见 docs/storage.md）
	StorageDriver        string        // 驱动名：local（默认）| s3 | 应用注册的驱动；空值按 local
	StoragePublicBaseURL string        // 对象访问 URL 前缀；空值使用驱动默认值
	StorageSigningSecret string        // 本地驱动预签名上传密钥；空值时由 JWTSecret 派生
	StorageUploadTTL     time.Duration // 预签名上传有效期；0 使用默认 15 分钟
	StorageS3Bucket      string
	StorageS3Region      string
	StorageS3Endpoint    string // S3 兼容服务地址（MinIO、R2 等）
	StorageS3PathStyle   bool   // 使用 endpoint/bucket/key 路径风格寻址

	// 解析错误按校验边界保留；Validate 汇总应用与数据库错误，
	// ValidateDatabase 汇总数据库错误，ValidateDatabaseDriver 校验离线类型。
	parseErrs         []string
	databaseParseErrs []string
}

// Load 从环境变量读取配置，提供合理默认值。
// 启动时尝试加载 .env（候选路径：./.env、../.env），已存在的环境变量不会被覆盖。
// godotenv.Load 在任一文件不存在时会整体返回错误且不加载后续文件，
// 因此逐个尝试并忽略「不存在」的错误，使从仓库根目录或 backend/ 运行都能命中。
//
// 环境变量「存在但格式非法」（如 ADMIN_SHUTDOWN_TIMEOUT_SECONDS=abc）按字段保存解析错误，
// 由 Validate / ValidateDatabase 在连接前拒绝。
func Load() *Config {
	for _, p := range []string{".env", "../.env"} {
		_ = godotenv.Load(p)
	}
	c := &Config{
		Env:         normalizeEnv(getenv("ADMIN_ENV", EnvDev)),
		Addr:        getenv("ADMIN_ADDR", ":8080"),
		DBDriver:    os.Getenv("ADMIN_DB_DRIVER"),
		DBDSN:       os.Getenv("ADMIN_DB_DSN"),
		JWTSecret:   getenv("ADMIN_JWT_SECRET", defaultJWTSecret),
		CORSOrigins: parseCORSOrigins(getenv("ADMIN_CORS_ORIGINS", "")),
		TrustProxy:  getenv("ADMIN_TRUST_PROXY", "") == "true",

		RefreshTokenEnabled: getenv("ADMIN_REFRESH_TOKEN_ENABLED", "") == "true",

		ShutdownTimeout:     defaultShutdownTimeout,
		DBMaxOpenConns:      defaultDBMaxOpenConns,
		DBMaxIdleConns:      defaultDBMaxIdleConns,
		JWTExpiresIn:        defaultJWTExpiresIn,
		JWTRefreshExpiresIn: defaultJWTRefreshExpiresIn,
		LoginMaxFailures:    defaultLoginMaxFailures,
		LoginLockoutSeconds: defaultLoginLockoutSeconds,
		UploadMaxBytes:      defaultUploadMaxBytes,
		AvatarMaxBytes:      defaultAvatarMaxBytes,

		UploadDir: getenv("ADMIN_UPLOAD_DIR", "./uploads"),

		StorageDriver:        strings.ToLower(strings.TrimSpace(getenv("ADMIN_STORAGE_DRIVER", defaultStorageDriver))),
		StoragePublicBaseURL: strings.TrimSpace(os.Getenv("ADMIN_STORAGE_PUBLIC_BASE_URL")),
		StorageSigningSecret: os.Getenv("ADMIN_STORAGE_SIGNING_SECRET"),
		StorageS3Bucket:      strings.TrimSpace(os.Getenv("ADMIN_STORAGE_S3_BUCKET")),
		StorageS3Region:      strings.TrimSpace(os.Getenv("ADMIN_STORAGE_S3_REGION")),
		StorageS3Endpoint:    strings.TrimSpace(os.Getenv("ADMIN_STORAGE_S3_ENDPOINT")),
		StorageS3PathStyle:   getenv("ADMIN_STORAGE_S3_PATH_STYLE", "") == "true",
	}

	// 解析数值/时长类配置：仅当环境变量「已设置」时尝试解析；
	// 解析失败按字段保留，由对应校验入口拒绝。
	c.JWTExpiresIn = c.getDuration("ADMIN_JWT_EXPIRES_SECONDS", defaultJWTExpiresIn)
	c.JWTRefreshExpiresIn = c.getDuration("ADMIN_JWT_REFRESH_EXPIRES_SECONDS", defaultJWTRefreshExpiresIn)
	c.ShutdownTimeout = c.getDuration("ADMIN_SHUTDOWN_TIMEOUT_SECONDS", defaultShutdownTimeout)
	c.MetricsToken = os.Getenv("ADMIN_METRICS_TOKEN")
	if c.DBDriver == "sqlite" {
		c.DBMaxOpenConns, c.DBMaxIdleConns = 4, 4
	}
	c.DBMaxOpenConns = c.getInt("ADMIN_DB_MAX_OPEN_CONNS", c.DBMaxOpenConns)
	c.DBMaxIdleConns = c.getInt("ADMIN_DB_MAX_IDLE_CONNS", c.DBMaxIdleConns)
	c.LoginMaxFailures = c.getInt("ADMIN_LOGIN_MAX_FAILURES", defaultLoginMaxFailures)
	c.LoginLockoutSeconds = c.getInt("ADMIN_LOGIN_LOCKOUT_SECONDS", defaultLoginLockoutSeconds)
	c.UploadMaxBytes = c.getInt64("ADMIN_UPLOAD_MAX_BYTES", defaultUploadMaxBytes)
	c.AvatarMaxBytes = c.getInt64("ADMIN_AVATAR_MAX_BYTES", defaultAvatarMaxBytes)
	c.StorageUploadTTL = c.getDuration("ADMIN_STORAGE_UPLOAD_TTL_SECONDS", defaultStorageUploadTTL)

	return c
}

// getInt 解析整数并按字段保存解析错误；缺省值用于未设置配置及后续范围检查。
func (c *Config) getInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		problem := fmt.Sprintf("%s 非法整数（got=%q）", key, v)
		switch key {
		case "ADMIN_DB_MAX_OPEN_CONNS", "ADMIN_DB_MAX_IDLE_CONNS":
			c.databaseParseErrs = append(c.databaseParseErrs, problem)
		default:
			c.parseErrs = append(c.parseErrs, problem)
		}
		return def
	}
	return n
}

// getInt64 同 getInt，返回 int64（用于字节数等大值场景）。
func (c *Config) getInt64(key string, def int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		c.parseErrs = append(c.parseErrs, fmt.Sprintf("%s 非法整数（got=%q）", key, v))
		return def
	}
	return n
}

// getDuration 把环境变量（秒）解析为 time.Duration；
// 未设置返回 def；存在但非法（Atoi 失败或 n <= 0）追加错误到 parseErrs 并返回 def。
func (c *Config) getDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		c.parseErrs = append(c.parseErrs, fmt.Sprintf("%s 非法整数（got=%q）", key, v))
		return def
	}
	if n <= 0 {
		c.parseErrs = append(c.parseErrs, fmt.Sprintf("%s 必须 > 0（got=%d）", key, n))
		return def
	}
	return time.Duration(n) * time.Second
}

// IsProd 判断是否为生产模式。
func (c *Config) IsProd() bool { return c.Env == EnvProd }

// Validate 对配置做三层校验：
//  1. 解析错误（Load 阶段收集）：环境变量存在但格式非法（非数字、越界等），
//     dev/prod 一致拒绝启动，避免静默回退默认值（需求 8.10/8.12）。
//  2. 范围校验（需求 8.10/8.12/Property 12）：越界即返回错误。
//  3. 安全校验：JWT/DSN/CORS 等敏感默认值在 prod 模式拒绝，dev 仅告警。
func (c *Config) Validate() error {
	var problems []string
	if len(c.parseErrs) > 0 {
		problems = append(problems, c.parseErrs...)
	}
	if err := c.ValidateDatabase(); err != nil {
		problems = append(problems, err.Error())
	}
	if err := c.validateRange(); err != nil {
		problems = append(problems, err.Error())
	}
	if len(problems) > 0 {
		return fmt.Errorf("配置校验失败:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return c.validateSecurity()
}

// validateRange 检查所有数值配置的范围（dev/prod 一致强制）。
// 越界即返回错误，避免本地误调导致行为偏离。
func (c *Config) validateRange() error {
	var problems []string

	stSecs := int(c.ShutdownTimeout / time.Second)
	if stSecs < minShutdownTimeoutSecs || stSecs > maxShutdownTimeoutSecs {
		problems = append(problems, fmt.Sprintf(
			"ADMIN_SHUTDOWN_TIMEOUT_SECONDS 越界（got=%ds, want=%d..%d）",
			stSecs, minShutdownTimeoutSecs, maxShutdownTimeoutSecs,
		))
	}
	if c.DBMaxOpenConns < minDBMaxOpenConns || c.DBMaxOpenConns > maxDBMaxOpenConns {
		problems = append(problems, fmt.Sprintf(
			"ADMIN_DB_MAX_OPEN_CONNS 越界（got=%d, want=%d..%d）",
			c.DBMaxOpenConns, minDBMaxOpenConns, maxDBMaxOpenConns,
		))
	}
	if c.DBMaxIdleConns < 0 || c.DBMaxIdleConns > c.DBMaxOpenConns {
		problems = append(problems, fmt.Sprintf(
			"ADMIN_DB_MAX_IDLE_CONNS 越界（got=%d, want=0..%d）",
			c.DBMaxIdleConns, c.DBMaxOpenConns,
		))
	}
	if c.JWTExpiresIn <= 0 {
		problems = append(problems, "ADMIN_JWT_EXPIRES_SECONDS 必须 > 0")
	}
	// 启用刷新令牌时，refresh 有效期必须 > access 有效期（否则刷新毫无意义）。
	if c.RefreshTokenEnabled && c.JWTRefreshExpiresIn <= c.JWTExpiresIn {
		problems = append(problems, "ADMIN_JWT_REFRESH_EXPIRES_SECONDS 必须大于 ADMIN_JWT_EXPIRES_SECONDS")
	}
	if c.LoginMaxFailures < 1 {
		problems = append(problems, "ADMIN_LOGIN_MAX_FAILURES 必须 ≥ 1")
	}
	if c.LoginLockoutSeconds < 1 {
		problems = append(problems, "ADMIN_LOGIN_LOCKOUT_SECONDS 必须 ≥ 1")
	}
	if c.UploadMaxBytes <= 0 {
		problems = append(problems, "ADMIN_UPLOAD_MAX_BYTES 必须 > 0")
	}
	if c.AvatarMaxBytes <= 0 {
		problems = append(problems, "ADMIN_AVATAR_MAX_BYTES 必须 > 0")
	}
	problems = append(problems, c.storageProblems()...)
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("配置范围校验失败:\n  - %s", strings.Join(problems, "\n  - "))
}

// StorageDriverOrDefault 返回存储驱动名；未配置时为 local。
func (c *Config) StorageDriverOrDefault() string {
	if c.StorageDriver == "" {
		return defaultStorageDriver
	}
	return c.StorageDriver
}

// StorageUploadTTLOrDefault 返回预签名上传有效期；未配置时为 15 分钟。
func (c *Config) StorageUploadTTLOrDefault() time.Duration {
	if c.StorageUploadTTL <= 0 {
		return defaultStorageUploadTTL
	}
	return c.StorageUploadTTL
}

// storageProblems 校验存储配置（dev/prod 一致强制）。驱动名本身是否已注册
// 由 adminapp 在创建应用时检查。
func (c *Config) storageProblems() []string {
	var problems []string
	driver := c.StorageDriverOrDefault()
	if strings.ContainsAny(driver, " /\\") {
		problems = append(problems, fmt.Sprintf("ADMIN_STORAGE_DRIVER 非法（got=%q）", driver))
	}
	if driver == StorageDriverS3 {
		if c.StorageS3Bucket == "" {
			problems = append(problems, "ADMIN_STORAGE_DRIVER=s3 时必须设置 ADMIN_STORAGE_S3_BUCKET")
		}
		if c.StorageS3Region == "" {
			problems = append(problems, "ADMIN_STORAGE_DRIVER=s3 时必须设置 ADMIN_STORAGE_S3_REGION（R2 使用 auto）")
		}
		if c.StorageS3Endpoint != "" && !isHTTPURL(c.StorageS3Endpoint) {
			problems = append(problems, fmt.Sprintf("ADMIN_STORAGE_S3_ENDPOINT 必须是 http(s) 地址（got=%q）", c.StorageS3Endpoint))
		}
	}
	if u := c.StoragePublicBaseURL; u != "" && !strings.HasPrefix(u, "/") && !isHTTPURL(u) {
		problems = append(problems, fmt.Sprintf("ADMIN_STORAGE_PUBLIC_BASE_URL 必须是 http(s) 地址或以 / 开头的路径（got=%q）", u))
	}
	if strings.HasPrefix(c.StoragePublicBaseURL, "//") {
		problems = append(problems, "ADMIN_STORAGE_PUBLIC_BASE_URL 不能使用协议相对地址 //")
	}
	if c.StorageSigningSecret != "" && len(c.StorageSigningSecret) < minStorageSecretLength {
		problems = append(problems, fmt.Sprintf("ADMIN_STORAGE_SIGNING_SECRET 至少 %d 字节", minStorageSecretLength))
	}
	if c.StorageUploadTTL < 0 || c.StorageUploadTTL > maxStorageUploadTTL {
		problems = append(problems, fmt.Sprintf("ADMIN_STORAGE_UPLOAD_TTL_SECONDS 越界（got=%ds, want=1..%d）",
			int(c.StorageUploadTTL/time.Second), int(maxStorageUploadTTL/time.Second)))
	}
	return problems
}

func isHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// validateSecurity 检查敏感默认值：prod 模式拒绝，dev 仅告警。
func (c *Config) validateSecurity() error {
	var problems []string

	if c.JWTSecret == defaultJWTSecret || strings.TrimSpace(c.JWTSecret) == "" {
		problems = append(problems, "ADMIN_JWT_SECRET 仍为默认值，请设置强随机密钥")
	}
	if c.DBDriver == "postgres" && c.DBDSN == developmentPostgresDSN {
		problems = append(problems, "ADMIN_DB_DSN 仍为默认本地连接串，请设置生产数据库连接")
	}
	if len(c.CORSOrigins) == 0 {
		problems = append(problems, "ADMIN_CORS_ORIGINS 未配置，生产环境必须显式指定允许的前端来源")
	}
	for _, o := range c.CORSOrigins {
		if o == "*" {
			problems = append(problems, "ADMIN_CORS_ORIGINS 包含通配 *，生产环境禁止")
		}
	}

	if len(problems) == 0 {
		return nil
	}

	if c.IsProd() {
		return fmt.Errorf("生产模式安全检查失败:\n  - %s", strings.Join(problems, "\n  - "))
	}
	// dev 模式：仅告警。
	fmt.Fprintf(os.Stderr, "[warn] 检测到不安全的开发默认值（dev 模式放行）:\n  - %s\n", strings.Join(problems, "\n  - "))
	return nil
}

// CORSOriginsOrDefault 返回 CORS 来源；dev 模式下未配置时回退到通配，便于本地开发。
func (c *Config) CORSOriginsOrDefault() []string {
	if len(c.CORSOrigins) > 0 {
		return c.CORSOrigins
	}
	if c.IsProd() {
		return nil
	}
	return []string{"*"}
}

func normalizeEnv(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case EnvProd, "production":
		return EnvProd
	default:
		return EnvDev
	}
}

func parseCORSOrigins(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Clone returns a deep copy of slice fields so later caller mutations cannot
// change the configuration stored by the application.
func (c Config) Clone() Config {
	out := c
	if c.CORSOrigins != nil {
		out.CORSOrigins = append([]string(nil), c.CORSOrigins...)
	}
	if c.parseErrs != nil {
		out.parseErrs = append([]string(nil), c.parseErrs...)
	}
	if c.databaseParseErrs != nil {
		out.databaseParseErrs = append([]string(nil), c.databaseParseErrs...)
	}
	return out
}

// ValidateDatabase checks database connection settings used by migrate and
// application startup. It does not apply JWT or CORS production rules.
func (c *Config) ValidateDatabaseDriver() error {
	if c.DBDriver != "postgres" && c.DBDriver != "mysql" && c.DBDriver != "mariadb" && c.DBDriver != "sqlite" {
		return fmt.Errorf("ADMIN_DB_DRIVER must be postgres, mysql, mariadb or sqlite (got=%q)", c.DBDriver)
	}
	return nil
}

func (c *Config) ValidateDatabase() error {
	if c == nil {
		return fmt.Errorf("配置校验失败:\n  - 配置为空")
	}
	problems := append([]string(nil), c.databaseParseErrs...)
	if err := c.ValidateDatabaseDriver(); err != nil {
		problems = append(problems, err.Error())
	}
	if c.IsProd() && c.DBDriver == "postgres" && c.DBDSN == developmentPostgresDSN {
		problems = append(problems, "ADMIN_DB_DSN must select a production database")
	}
	if strings.TrimSpace(c.DBDSN) == "" {
		problems = append(problems, "ADMIN_DB_DSN 未配置")
	}
	if c.DBMaxOpenConns < minDBMaxOpenConns || c.DBMaxOpenConns > maxDBMaxOpenConns {
		problems = append(problems, fmt.Sprintf(
			"ADMIN_DB_MAX_OPEN_CONNS 越界（got=%d, want=%d..%d）",
			c.DBMaxOpenConns, minDBMaxOpenConns, maxDBMaxOpenConns,
		))
	}
	if c.DBMaxIdleConns < 0 || c.DBMaxIdleConns > c.DBMaxOpenConns {
		problems = append(problems, fmt.Sprintf(
			"ADMIN_DB_MAX_IDLE_CONNS 越界（got=%d, want=0..%d）",
			c.DBMaxIdleConns, c.DBMaxOpenConns,
		))
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("数据库配置校验失败:\n  - %s", strings.Join(problems, "\n  - "))
}

// LogValue reports configuration for structured logs without secrets.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("env", c.Env),
		slog.String("addr", c.Addr),
		slog.String("db_driver", c.DBDriver),
		slog.String("db_dsn", "[redacted]"),
		slog.String("jwt_secret", "[redacted]"),
		slog.Duration("jwt_expires_in", c.JWTExpiresIn),
		slog.Int("cors_origin_count", len(c.CORSOrigins)),
		slog.Bool("trust_proxy", c.TrustProxy),
		slog.Bool("refresh_token_enabled", c.RefreshTokenEnabled),
		slog.Duration("shutdown_timeout", c.ShutdownTimeout),
		slog.Int("db_max_open_conns", c.DBMaxOpenConns),
		slog.Int("db_max_idle_conns", c.DBMaxIdleConns),
		slog.Int("login_max_failures", c.LoginMaxFailures),
		slog.Int("login_lockout_seconds", c.LoginLockoutSeconds),
		slog.Int64("upload_max_bytes", c.UploadMaxBytes),
		slog.Int64("avatar_max_bytes", c.AvatarMaxBytes),
		slog.String("upload_dir", c.UploadDir),
		slog.String("storage_driver", c.StorageDriverOrDefault()),
		slog.String("storage_public_base_url", c.StoragePublicBaseURL),
		slog.Bool("storage_signing_secret_set", c.StorageSigningSecret != ""),
		slog.Duration("storage_upload_ttl", c.StorageUploadTTLOrDefault()),
		slog.String("storage_s3_bucket", c.StorageS3Bucket),
		slog.String("storage_s3_region", c.StorageS3Region),
		slog.String("storage_s3_endpoint", c.StorageS3Endpoint),
		slog.Bool("storage_s3_path_style", c.StorageS3PathStyle),
	)
}
