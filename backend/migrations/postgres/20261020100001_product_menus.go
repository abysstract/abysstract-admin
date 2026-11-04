package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/pressly/goose/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func ProductMenusMigration() *goose.Migration {
	return goose.NewGoMigration(20261020100001, &goose.GoFunc{RunTx: upProductMenus}, nil)
}

func upProductMenus(ctx context.Context, tx *sql.Tx) error {
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: tx}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		return err
	}
	return applyMenus(ctx, db)
}

const (
	objectTypeMenu = "menu"
	objectTypeRole = "role"

	keyDirectory = "menu.product.directory"
	keyPage      = "menu.product.page"
	keyList      = "menu.product.button.list"
	keyAdd       = "menu.product.button.add"
	keyEdit      = "menu.product.button.edit"
	keyRemove    = "menu.product.button.remove"
	keyReader    = "role.product.reader"
	keyEditor    = "role.product.editor"

	roleReaderKey = "biz_product_reader"
	roleEditorKey = "biz_product_editor"

	permList   = "biz:product:list"
	permAdd    = "biz:product:add"
	permEdit   = "biz:product:edit"
	permRemove = "biz:product:remove"

	pageComponent = "biz/product/index"
	dataScopeDept = 3
)

func applyMenus(ctx context.Context, db *gorm.DB) error {
	tx := db.WithContext(ctx)

	directoryID, err := ensureMenu(tx, boundMenu{
		ObjectKey: keyDirectory,
		MenuName:  "商品管理",
		ParentKey: "",
		OrderNum:  50,
		Path:      "biz",
		Component: "",
		MenuType:  "M",
		Perms:     "",
		Icon:      "product",
		parentID:  0,
	}, 0)
	if err != nil {
		return err
	}
	pageID, err := ensureMenu(tx, boundMenu{
		ObjectKey: keyPage,
		MenuName:  "商品列表",
		OrderNum:  1,
		Path:      "product",
		Component: pageComponent,
		MenuType:  "C",
		Perms:     permList,
		Icon:      "product",
	}, directoryID)
	if err != nil {
		return err
	}
	listID, err := ensureMenu(tx, boundMenu{ObjectKey: keyList, MenuName: "商品查询", OrderNum: 1, MenuType: "F", Perms: permList}, pageID)
	if err != nil {
		return err
	}
	addID, err := ensureMenu(tx, boundMenu{ObjectKey: keyAdd, MenuName: "商品新增", OrderNum: 2, MenuType: "F", Perms: permAdd}, pageID)
	if err != nil {
		return err
	}
	editID, err := ensureMenu(tx, boundMenu{ObjectKey: keyEdit, MenuName: "商品修改", OrderNum: 3, MenuType: "F", Perms: permEdit}, pageID)
	if err != nil {
		return err
	}
	removeID, err := ensureMenu(tx, boundMenu{ObjectKey: keyRemove, MenuName: "商品删除", OrderNum: 4, MenuType: "F", Perms: permRemove}, pageID)
	if err != nil {
		return err
	}

	readerID, err := ensureRole(tx, keyReader, roleReaderKey, "商品只读", dataScopeDept)
	if err != nil {
		return err
	}
	editorID, err := ensureRole(tx, keyEditor, roleEditorKey, "商品编辑", dataScopeDept)
	if err != nil {
		return err
	}

	readerMenus := []int64{directoryID, pageID, listID}
	editorMenus := []int64{directoryID, pageID, listID, addID, editID, removeID}
	if err := ensureRoleMenus(tx, readerID, readerMenus); err != nil {
		return err
	}
	return ensureRoleMenus(tx, editorID, editorMenus)
}

type boundMenu struct {
	ObjectKey string
	MenuName  string
	ParentKey string
	OrderNum  int
	Path      string
	Component string
	MenuType  string
	Perms     string
	Icon      string
	parentID  int64
}

func ensureMenu(tx *gorm.DB, menu boundMenu, parentID int64) (int64, error) {
	boundID, ok, err := lookupBinding(tx, menu.ObjectKey, objectTypeMenu)
	if err != nil {
		return 0, err
	}
	if ok {
		liveID, live, err := scanOptionalID(tx, `SELECT id FROM sys_menu WHERE id = ?`, boundID)
		if err != nil {
			return 0, err
		}
		if !live {
			return 0, fmt.Errorf("product migration: bound menu %s id %d is missing", menu.ObjectKey, boundID)
		}
		return liveID, nil
	}
	id, err := insertReturningID(tx, `
		INSERT INTO sys_menu (menu_name, parent_id, order_num, path, component, menu_type, perms, icon)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`,
		menu.MenuName, parentID, menu.OrderNum, menu.Path, menu.Component, menu.MenuType, menu.Perms, menu.Icon)
	if err != nil {
		return 0, fmt.Errorf("product migration: insert menu %s: %w", menu.ObjectKey, err)
	}
	if err := insertBinding(tx, menu.ObjectKey, objectTypeMenu, id); err != nil {
		return 0, err
	}
	return id, nil
}

func ensureRole(tx *gorm.DB, objectKey, roleKey, roleName string, dataScope int16) (int64, error) {
	boundID, ok, err := lookupBinding(tx, objectKey, objectTypeRole)
	if err != nil {
		return 0, err
	}
	if ok {
		liveID, live, err := scanOptionalID(tx, `SELECT id FROM sys_role WHERE id = ? AND del_flag = 0`, boundID)
		if err != nil {
			return 0, err
		}
		if !live {
			return 0, fmt.Errorf("product migration: bound role %s id %d is missing or deleted", objectKey, boundID)
		}
		return liveID, nil
	}
	conflictID, conflict, err := scanOptionalID(tx, `SELECT id FROM sys_role WHERE role_key = ? AND del_flag = 0`, roleKey)
	if err != nil {
		return 0, err
	}
	if conflict {
		return 0, fmt.Errorf("product migration: role key %s already exists without module binding (id=%d)", roleKey, conflictID)
	}
	id, err := insertReturningID(tx, `
		INSERT INTO sys_role (role_name, role_key, role_sort, data_scope, remark)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id`,
		roleName, roleKey, 100, dataScope, "业务商品角色")
	if err != nil {
		return 0, fmt.Errorf("product migration: insert role %s: %w", roleKey, err)
	}
	if err := insertBinding(tx, objectKey, objectTypeRole, id); err != nil {
		return 0, err
	}
	return id, nil
}

func ensureRoleMenus(tx *gorm.DB, roleID int64, menuIDs []int64) error {
	for _, menuID := range menuIDs {
		if err := tx.Exec(
			`INSERT INTO sys_role_menu (role_id, menu_id) VALUES (?, ?) ON CONFLICT DO NOTHING`,
			roleID, menuID,
		).Error; err != nil {
			return fmt.Errorf("product migration: bind role %d menu %d: %w", roleID, menuID, err)
		}
	}
	return nil
}

func lookupBinding(tx *gorm.DB, objectKey, objectType string) (int64, bool, error) {
	var systemID int64
	err := tx.Raw(
		`SELECT system_id FROM biz_admin_binding WHERE object_key = ? AND object_type = ?`,
		objectKey, objectType,
	).Scan(&systemID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if systemID == 0 {
		return 0, false, nil
	}
	return systemID, true, nil
}

func insertBinding(tx *gorm.DB, objectKey, objectType string, systemID int64) error {
	if err := tx.Exec(
		`INSERT INTO biz_admin_binding (object_key, object_type, system_id) VALUES (?, ?, ?)`,
		objectKey, objectType, systemID,
	).Error; err != nil {
		return fmt.Errorf("product migration: bind %s: %w", objectKey, err)
	}
	return nil
}

func insertReturningID(tx *gorm.DB, sql string, args ...any) (int64, error) {
	var id int64
	if err := tx.Raw(sql, args...).Scan(&id).Error; err != nil {
		return 0, err
	}
	if id == 0 {
		return 0, fmt.Errorf("insert returned no id")
	}
	return id, nil
}

func scanOptionalID(tx *gorm.DB, sql string, args ...any) (int64, bool, error) {
	var id int64
	err := tx.Raw(sql, args...).Scan(&id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if id == 0 {
		return 0, false, nil
	}
	return id, true, nil
}
