//go:build integration

package gen

import (
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

func TestGeneratedDictControllersAndAdapterConstructorValidation(t *testing.T) {
	baseRepo := repo.New(testutil.OpenDB(t))

	if _, err := NewDictTypeController(DictTypeControllerDeps{}); err == nil {
		t.Fatal("DictType controller accepted nil BaseRepo")
	}
	if _, err := NewDictDataController(DictDataControllerDeps{Store: testutil.Store(t, baseRepo)}); err == nil {
		t.Fatal("DictData controller accepted nil injected Repository")
	}

	dictType, err := NewDictTypeController(DictTypeControllerDeps{Store: testutil.Store(t, baseRepo)})
	if err != nil {
		t.Fatalf("NewDictTypeController: %v", err)
	}
	dictData, err := NewDictDataController(DictDataControllerDeps{
		Store:      testutil.Store(t, baseRepo),
		Repository: &dictDataInjectedRepository{inner: NewDictDataRepository()},
	})
	if err != nil {
		t.Fatalf("NewDictDataController: %v", err)
	}

	if _, err := NewDictServiceAdapter(nil, dictData); err == nil {
		t.Fatal("DictServiceAdapter accepted nil DictType controller")
	}
	if _, err := NewDictServiceAdapter(dictType, nil); err == nil {
		t.Fatal("DictServiceAdapter accepted nil DictData controller")
	}
	if _, err := NewDictServiceAdapter(dictType, dictData); err != nil {
		t.Fatalf("NewDictServiceAdapter: %v", err)
	}
}
