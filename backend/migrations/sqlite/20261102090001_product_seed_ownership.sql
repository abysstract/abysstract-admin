-- +goose Up
INSERT INTO sys_module_binding (module_name, object_key, object_type, system_id)
SELECT 'product',
  CASE object_key
    WHEN 'role.product.reader' THEN 'biz_product_reader'
    WHEN 'role.product.editor' THEN 'biz_product_editor'
    ELSE object_key
  END,
  object_type, system_id
FROM product_binding;

-- +goose Down
DELETE FROM sys_module_binding WHERE module_name = 'product';
