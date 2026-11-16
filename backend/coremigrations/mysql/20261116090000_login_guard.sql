-- +goose Up
-- 登录验证码与按 IP 计数（见 docs/configuration.md#登录安全）。
-- sys_login_ip_attempt：每个来源 IP（IPv6 按 /64）一行，记录窗口内的失败次数；
--   window_until 过后下次失败重新计数，过期行在记录失败时清理。登录成功不清零。
-- sys_captcha_used：已用于登录的验证码挑战 ID（挑战内容签名后由客户端携带，服务端不保存答案），
--   防止同一挑战重复使用；expires_at 后可删除。
CREATE TABLE sys_login_ip_attempt (
  ip           VARCHAR(64) COLLATE utf8mb4_bin PRIMARY KEY,
  fail_count   INT NOT NULL DEFAULT 0,
  window_until DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
CREATE INDEX idx_login_ip_attempt_window ON sys_login_ip_attempt (window_until);

CREATE TABLE sys_captcha_used (
  id         VARCHAR(64) COLLATE utf8mb4_bin PRIMARY KEY,
  expires_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
CREATE INDEX idx_captcha_used_expires ON sys_captcha_used (expires_at);

-- +goose Down
DROP TABLE sys_captcha_used;
DROP TABLE sys_login_ip_attempt;
