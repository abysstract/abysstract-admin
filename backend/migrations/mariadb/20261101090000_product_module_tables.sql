-- +goose Up
ALTER TABLE biz_product RENAME TO product_item;
ALTER TABLE biz_product_event RENAME TO product_event;
ALTER TABLE biz_admin_binding RENAME TO product_binding;

-- +goose Down
ALTER TABLE product_binding RENAME TO biz_admin_binding;
ALTER TABLE product_event RENAME TO biz_product_event;
ALTER TABLE product_item RENAME TO biz_product;
