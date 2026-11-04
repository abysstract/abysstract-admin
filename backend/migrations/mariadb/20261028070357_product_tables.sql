-- +goose Up
-- 商品业务表。版本 1 建表；由迁移执行器在同一事务中应用。
CREATE TABLE IF NOT EXISTS biz_product (
  id            BIGINT AUTO_INCREMENT PRIMARY KEY,
  product_no   VARCHAR(64) COLLATE utf8mb4_bin  NOT NULL,
  name         VARCHAR(100) NOT NULL,
  price_cents  BIGINT       NOT NULL DEFAULT 0 CHECK (price_cents >= 0),
  dept_id       BIGINT       NOT NULL,
  FOREIGN KEY (dept_id) REFERENCES sys_dept(id),
  create_by     BIGINT       NOT NULL,
  remark        VARCHAR(500) NOT NULL DEFAULT '',
  create_time   DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  update_time   DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE UNIQUE INDEX uq_biz_product_product_no
  ON biz_product (product_no);
CREATE INDEX idx_biz_product_dept_id
  ON biz_product (dept_id);
CREATE INDEX idx_biz_product_create_by
  ON biz_product (create_by);

CREATE TABLE IF NOT EXISTS biz_product_event (
  id           BIGINT AUTO_INCREMENT PRIMARY KEY,
  product_id  BIGINT       NOT NULL,
  actor_id     BIGINT       NOT NULL,
  action       VARCHAR(32)  NOT NULL,
  created_at   DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE INDEX idx_biz_product_event_product_id
  ON biz_product_event (product_id);

CREATE TABLE IF NOT EXISTS biz_admin_binding (
  object_key   VARCHAR(128) COLLATE utf8mb4_bin PRIMARY KEY,
  object_type  VARCHAR(32)  NOT NULL,
  system_id    BIGINT       NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE UNIQUE INDEX uq_biz_admin_binding_type_id
  ON biz_admin_binding (object_type, system_id);

-- +goose Down

-- This installation migration is forward-only.
