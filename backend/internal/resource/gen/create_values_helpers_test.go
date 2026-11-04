package gen

import (
	"errors"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/database"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
)

type createCallbackModel struct {
	model.SysDictType
	Calls []string `gorm:"-"`
	Fail  string   `gorm:"-"`
}

func (*createCallbackModel) TableName() string { return "sys_dict_type" }

var errCreateModelHook = errors.New("model callback failed")

func (m *createCallbackModel) BeforeCreate(*gorm.DB) error {
	m.Calls = append(m.Calls, "before")
	m.DictName = "model-before"
	if m.Fail == "before" {
		return errCreateModelHook
	}
	return nil
}

func (m *createCallbackModel) AfterCreate(tx *gorm.DB) error {
	m.Calls = append(m.Calls, "after")
	// Native callbacks observe the values supplied by GORM and its driver.
	if m.ID == 0 || m.Status != 1 {
		return errors.New("model AfterCreate requires generated ID and GORM defaults")
	}
	if database.DriverOf(tx) == "mariadb" && (m.CreateTime.IsZero() || m.UpdateTime.IsZero()) {
		return errors.New("MariaDB native RETURNING requires database times in AfterCreate")
	}
	if err := tx.Exec("INSERT INTO create_case_events (id) VALUES (?)", m.ID).Error; err != nil {
		return err
	}
	if m.Fail == "after" {
		return errCreateModelHook
	}
	return nil
}
