package moduleseed

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Upgrade changes declaration-owned defaults while preserving administrator edits.
// The caller supplies a transaction, just as for Install and Remove.
func Upgrade(tx *gorm.DB, before, after Spec) error {
	if err := before.Validate(); err != nil {
		return err
	}
	if err := after.Validate(); err != nil {
		return err
	}
	if before.Module != after.Module {
		return fmt.Errorf("seed: upgrade module differs")
	}
	var bindings []binding
	if err := tx.Where("module_name = ?", before.Module).Find(&bindings).Error; err != nil {
		return err
	}
	owned := map[string]binding{}
	for _, b := range bindings {
		owned[b.ObjectKey] = b
	}
	nextMenus := map[string]Menu{}
	for _, m := range after.Menus {
		nextMenus[m.Key] = m
	}
	nextRoles := map[string]Role{}
	for _, r := range after.Roles {
		nextRoles[r.Key] = r
	}
	var removedMenus, removedRoles []int64
	for _, m := range before.Menus {
		b, ok := owned[m.Key]
		if !ok {
			return fmt.Errorf("seed: missing ownership for %s", m.Key)
		}
		if _, keep := nextMenus[m.Key]; !keep {
			removedMenus = append(removedMenus, b.SystemID)
		}
	}
	for _, r := range before.Roles {
		b, ok := owned[r.Key]
		if !ok {
			return fmt.Errorf("seed: missing ownership for %s", r.Key)
		}
		if _, keep := nextRoles[r.Key]; !keep {
			removedRoles = append(removedRoles, b.SystemID)
		}
	}
	// Create additions before resolving parent changes to newly introduced menus.
	if err := Install(tx, after); err != nil {
		return err
	}
	bindings = nil
	if err := tx.Where("module_name = ?", before.Module).Find(&bindings).Error; err != nil {
		return err
	}
	for _, b := range bindings {
		owned[b.ObjectKey] = b
	}
	if err := upgradeMenus(tx, before.Menus, nextMenus, owned); err != nil {
		return err
	}

	if err := upgradeRoles(tx, before.Roles, nextRoles, owned); err != nil {
		return err
	}
	return removeRetiredObjects(tx, before.Module, removedMenus, removedRoles)
}

func removeRetiredObjects(tx *gorm.DB, module string, removedMenus, removedRoles []int64) error {
	if len(removedMenus) > 0 {
		var children []menuRow
		if err := tx.Table("sys_menu").Where("parent_id IN ? AND id NOT IN ?", removedMenus, removedMenus).Find(&children).Error; err != nil {
			return err
		}
		if len(children) > 0 {
			return fmt.Errorf("seed %s: removed menu has retained child %d", module, children[0].ID)
		}
		if err := tx.Table("sys_role_menu").Where("menu_id IN ?", removedMenus).Delete(&struct{}{}).Error; err != nil {
			return err
		}
		if err := tx.Table("sys_menu").Where("id IN ?", removedMenus).Delete(&menuRow{}).Error; err != nil {
			return err
		}
		if err := tx.Where("module_name = ? AND object_type = ? AND system_id IN ?", module, "menu", removedMenus).Delete(&binding{}).Error; err != nil {
			return err
		}
	}
	if len(removedRoles) > 0 {
		for _, table := range []string{"sys_user_role", "sys_role_dept", "sys_role_menu"} {
			if err := tx.Table(table).Where("role_id IN ?", removedRoles).Delete(&struct{}{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Table("sys_role").Where("id IN ?", removedRoles).Delete(&roleRow{}).Error; err != nil {
			return err
		}
		if err := tx.Where("module_name = ? AND object_type = ? AND system_id IN ?", module, "role", removedRoles).Delete(&binding{}).Error; err != nil {
			return err
		}
	}
	return nil
}

func upgradeMenus(tx *gorm.DB, previous []Menu, nextMenus map[string]Menu, owned map[string]binding) error {
	for _, old := range previous {
		next, keep := nextMenus[old.Key]
		if !keep {
			continue
		}
		id := owned[old.Key].SystemID
		var row menuRow
		if err := tx.Table("sys_menu").First(&row, id).Error; err != nil {
			return err
		}
		updates := map[string]any{}
		for _, field := range []struct {
			column             string
			current, old, next any
		}{
			{"menu_name", row.MenuName, old.Name, next.Name},
			{"parent_id", row.ParentID, owned[old.Parent].SystemID, owned[next.Parent].SystemID},
			{"order_num", row.OrderNum, old.Order, next.Order},
			{"path", row.Path, old.Path, next.Path},
			{"component", row.Component, old.Component, next.Component},
			{"menu_type", row.MenuType, old.Type, next.Type},
			{"perms", row.Perms, old.Permission, next.Permission},
			{"icon", row.Icon, old.Icon, next.Icon},
		} {
			if field.current == field.old && field.old != field.next {
				updates[field.column] = field.next
			}
		}
		if len(updates) > 0 {
			if err := tx.Table("sys_menu").Where("id = ?", id).Updates(updates).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

func upgradeRoles(tx *gorm.DB, previous []Role, nextRoles map[string]Role, owned map[string]binding) error {
	for _, old := range previous {
		next, keep := nextRoles[old.Key]
		if !keep {
			continue
		}
		id := owned[old.Key].SystemID
		var row roleRow
		if err := tx.Table("sys_role").First(&row, id).Error; err != nil {
			return err
		}
		updates := map[string]any{}
		if row.RoleName == old.Name && old.Name != next.Name {
			updates["role_name"] = next.Name
		}
		if row.DataScope == old.DataScope && old.DataScope != next.DataScope {
			updates["data_scope"] = next.DataScope
		}
		if len(updates) > 0 {
			if err := tx.Table("sys_role").Where("id = ?", id).Updates(updates).Error; err != nil {
				return err
			}
		}
		oldGrants, newGrants := map[string]bool{}, map[string]bool{}
		for _, key := range old.Menus {
			oldGrants[key] = true
		}
		for _, key := range next.Menus {
			newGrants[key] = true
		}
		for key := range oldGrants {
			if !newGrants[key] {
				if err := tx.Table("sys_role_menu").Where("role_id = ? AND menu_id = ?", id, owned[key].SystemID).Delete(&struct{}{}).Error; err != nil {
					return err
				}
			}
		}
		for key := range newGrants {
			if !oldGrants[key] {
				relation := map[string]any{"role_id": id, "menu_id": owned[key].SystemID}
				if err := tx.Table("sys_role_menu").Clauses(clause.OnConflict{DoNothing: true}).Create(relation).Error; err != nil {
					return err
				}
			}
		}
	}
	return nil
}
