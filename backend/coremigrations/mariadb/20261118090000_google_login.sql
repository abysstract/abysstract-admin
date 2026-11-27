-- +goose Up
-- 外部身份绑定（Google 登录，见 docs/configuration.md#google-登录）。
-- sys_user_identity：外部账号与后台用户的绑定，(provider, subject) 唯一，subject 是 Google 的 sub
--   （不随邮箱变化）；每个用户在每个 provider 下最多绑定一个账号。首次登录时按邮箱匹配后台用户并写入，
--   之后只认 subject，改邮箱不影响登录。
CREATE TABLE sys_user_identity (
  provider    VARCHAR(32) COLLATE utf8mb4_bin NOT NULL,
  subject     VARCHAR(255) COLLATE utf8mb4_bin NOT NULL,
  user_id     BIGINT NOT NULL,
  email       VARCHAR(128) NOT NULL DEFAULT '',
  create_time DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (provider, subject)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
CREATE UNIQUE INDEX uk_user_identity_user ON sys_user_identity (user_id, provider);

-- +goose Down
DROP TABLE sys_user_identity;
