package moduleprobe

import (
	"context"
	"log/slog"
	"testing"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/module"
)

func TestDefinitionUsesPublicModuleTypes(t *testing.T) {
	definition := DefinitionFor(Config{Prefix: "product"})
	if definition.Name != "product" || definition.Create == nil {
		t.Fatal("business definition did not retain its public shape")
	}
	instance, err := definition.Create(context.Background(), module.Runtime{
		DB:     &gorm.DB{},
		Logger: slog.Default(),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(instance.Services) != 1 || instance.Services[0].Register == nil || instance.Close == nil {
		t.Fatal("business module did not return a service and cleanup function")
	}
	if err := instance.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
