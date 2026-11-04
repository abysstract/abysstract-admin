package gen

import (
	"errors"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
)

// Both paths delete a zero model, so model callbacks receive its zero ID.
// Resource hooks separately receive the rows from the scoped locking read.
type deleteHookContextKey struct{}

type deleteCallbackModel struct{ model.SysDictType }

func (*deleteCallbackModel) TableName() string { return "sys_dict_type" }

var errDeleteModelHook = errors.New("model delete callback")

func (m *deleteCallbackModel) BeforeDelete(tx *gorm.DB) error {
	if m.ID != 0 {
		return errors.New("expected zero model")
	}
	if err := tx.Exec("INSERT INTO delete_case_events VALUES ('before')").Error; err != nil {
		return err
	}
	if tx.Statement.Context.Value(deleteHookContextKey{}) == "before" {
		return errDeleteModelHook
	}
	return nil
}

func (*deleteCallbackModel) AfterDelete(tx *gorm.DB) error {
	if err := tx.Exec("INSERT INTO delete_case_events VALUES ('after')").Error; err != nil {
		return err
	}
	if tx.Statement.Context.Value(deleteHookContextKey{}) == "after" {
		return errDeleteModelHook
	}
	return nil
}
