-- +goose Up
CREATE TABLE sys_module_binding (
  module_name VARCHAR(64) NOT NULL,
  object_key VARCHAR(128) NOT NULL,
  object_type VARCHAR(32) NOT NULL,
  system_id BIGINT NOT NULL,
  PRIMARY KEY (module_name, object_key),
  UNIQUE (object_type, system_id)
);

-- +goose Down
DROP TABLE sys_module_binding;
