// Package moduleseed installs module-owned menus and optional example roles.
// Callers supply the migration transaction; all errors propagate to that caller.
package moduleseed

import (
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
)

type Menu struct {
	Key        string `json:"key"`
	Parent     string `json:"parent,omitempty"`
	Name       string `json:"name"`
	Path       string `json:"path,omitempty"`
	Component  string `json:"component,omitempty"`
	Type       string `json:"type"`
	Permission string `json:"permission,omitempty"`
	Icon       string `json:"icon,omitempty"`
	Order      int    `json:"order,omitempty"`
}
type Role struct {
	Key       string   `json:"key"`
	Name      string   `json:"name"`
	DataScope int16    `json:"dataScope"`
	Menus     []string `json:"menus"`
}
type Spec struct {
	Module string `json:"module"`
	Menus  []Menu `json:"menus"`
	Roles  []Role `json:"roles,omitempty"`
}

type binding struct {
	ModuleName string `gorm:"primaryKey"`
	ObjectKey  string `gorm:"primaryKey"`
	ObjectType string
	SystemID   int64
}

func (binding) TableName() string { return "sys_module_binding" }

type menuRow struct {
	ID        int64
	MenuName  string
	ParentID  int64
	OrderNum  int
	Path      string
	Component string
	MenuType  string
	Perms     string
	Icon      string
	Visible   int16
	Status    int16
}
type roleRow struct {
	ID        int64
	RoleName  string
	RoleKey   string
	DataScope int16
	Status    int16
}

// Parse validates the external Seed declaration before preparing migrations.
func Parse(data []byte) (Spec, error) {
	var spec Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		return spec, err
	}
	return spec, spec.Validate()
}

func (s Spec) Validate() error {
	if s.Module == "" {
		return fmt.Errorf("seed: module is required")
	}
	keys := map[string]bool{}
	menuKeys := map[string]bool{}
	for _, m := range s.Menus {
		if m.Key == "" || keys[m.Key] || m.Name == "" {
			return fmt.Errorf("seed %s: duplicate or incomplete menu %q", s.Module, m.Key)
		}
		if m.Type != "M" && m.Type != "C" && m.Type != "F" {
			return fmt.Errorf("seed %s: invalid menu type %q", s.Module, m.Type)
		}
		if m.Parent != "" && !keys[m.Parent] {
			return fmt.Errorf("seed %s: parent %s must precede %s", s.Module, m.Parent, m.Key)
		}
		keys[m.Key] = true
		menuKeys[m.Key] = true
	}
	return s.validateRoles(keys, menuKeys)
}

// Install reuses owned objects and leaves administrator edits and grants intact.
func Install(tx *gorm.DB, spec Spec) error {
	if err := spec.Validate(); err != nil {
		return err
	}
	var existing []binding
	if err := tx.Where("module_name = ?", spec.Module).Find(&existing).Error; err != nil {
		return err
	}
	owned := map[string]binding{}
	for _, b := range existing {
		owned[b.ObjectKey] = b
	}
	menuIDs := map[string]int64{}
	for _, m := range spec.Menus {
		if b, ok := owned[m.Key]; ok {
			if b.ObjectType != "menu" {
				return fmt.Errorf("seed: %s ownership type conflict", m.Key)
			}
			var row menuRow
			if err := tx.Table("sys_menu").First(&row, b.SystemID).Error; err != nil {
				return err
			}
			menuIDs[m.Key] = b.SystemID
			continue
		}
		row := menuRow{MenuName: m.Name, ParentID: menuIDs[m.Parent], OrderNum: m.Order, Path: m.Path, Component: m.Component, MenuType: m.Type, Perms: m.Permission, Icon: m.Icon, Visible: 1, Status: 1}
		if err := tx.Table("sys_menu").Create(&row).Error; err != nil {
			return err
		}
		if err := tx.Create(&binding{ModuleName: spec.Module, ObjectKey: m.Key, ObjectType: "menu", SystemID: row.ID}).Error; err != nil {
			return err
		}
		menuIDs[m.Key] = row.ID
	}
	for _, r := range spec.Roles {
		if b, ok := owned[r.Key]; ok {
			if b.ObjectType != "role" {
				return fmt.Errorf("seed: %s ownership type conflict", r.Key)
			}
			var row roleRow
			if err := tx.Table("sys_role").Where("id = ? AND del_flag = 0", b.SystemID).First(&row).Error; err != nil {
				return err
			}
			continue
		}
		row := roleRow{RoleName: r.Name, RoleKey: r.Key, DataScope: r.DataScope, Status: 1}
		if err := tx.Table("sys_role").Create(&row).Error; err != nil {
			return err
		}
		if err := tx.Create(&binding{ModuleName: spec.Module, ObjectKey: r.Key, ObjectType: "role", SystemID: row.ID}).Error; err != nil {
			return err
		}
		for _, key := range r.Menus {
			relation := map[string]any{"role_id": row.ID, "menu_id": menuIDs[key]}
			if err := tx.Table("sys_role_menu").Create(relation).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// Remove deletes only owned system records and their authorization relations.
// Business tables and ordinary administrator-created roles are retained.
func Remove(tx *gorm.DB, module string) error {
	var owned []binding
	if err := tx.Where("module_name = ?", module).Find(&owned).Error; err != nil {
		return err
	}
	var menuIDs, roleIDs []int64
	for _, b := range owned {
		switch b.ObjectType {
		case "menu":
			menuIDs = append(menuIDs, b.SystemID)
		case "role":
			roleIDs = append(roleIDs, b.SystemID)
		default:
			return fmt.Errorf("seed: unsupported ownership type %s", b.ObjectType)
		}
	}
	if len(menuIDs) > 0 {
		var children []menuRow
		if err := tx.Table("sys_menu").Where("parent_id IN ? AND id NOT IN ?", menuIDs, menuIDs).Find(&children).Error; err != nil {
			return err
		}
		if len(children) > 0 {
			return fmt.Errorf("seed %s: menu %d has an externally owned child %d", module, children[0].ParentID, children[0].ID)
		}
		if err := tx.Table("sys_role_menu").Where("menu_id IN ?", menuIDs).Delete(&struct{}{}).Error; err != nil {
			return err
		}
		if err := tx.Table("sys_menu").Where("id IN ?", menuIDs).Delete(&menuRow{}).Error; err != nil {
			return err
		}
	}
	if len(roleIDs) > 0 {
		for _, table := range []string{"sys_user_role", "sys_role_dept", "sys_role_menu"} {
			if err := tx.Table(table).Where("role_id IN ?", roleIDs).Delete(&struct{}{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Table("sys_role").Where("id IN ?", roleIDs).Delete(&roleRow{}).Error; err != nil {
			return err
		}
	}
	return tx.Where("module_name = ?", module).Delete(&binding{}).Error
}

func (s Spec) validateRoles(keys, menuKeys map[string]bool) error {
	for _, r := range s.Roles {
		if r.Key == "" || keys[r.Key] || r.Key == "admin" || r.Name == "" || r.DataScope < 1 || r.DataScope > 5 {
			return fmt.Errorf("seed %s: invalid role %q", s.Module, r.Key)
		}
		grants := map[string]bool{}
		for _, menu := range r.Menus {
			if grants[menu] {
				return fmt.Errorf("seed %s: duplicate menu %s for role %s", s.Module, menu, r.Key)
			}
			grants[menu] = true
			if !menuKeys[menu] {
				return fmt.Errorf("seed %s: unknown menu %s", s.Module, menu)
			}
		}
		keys[r.Key] = true
	}
	return nil
}
