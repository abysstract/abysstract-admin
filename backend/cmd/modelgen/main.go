// modelgen generates migrated tables for application modules.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/abysstract/abysstract-admin/backend/adminconfig"
	"github.com/abysstract/abysstract-admin/backend/database"
	"github.com/abysstract/abysstract-admin/backend/internal/codegen/modelgen"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "modelgen:", err)
		os.Exit(1)
	}
}

func run() error {
	selected := flag.String("module", "", "generate one module from the inventory")
	inventory := flag.String("inventory", "internal/codegen/modules.json", "module generation inventory")
	flag.Parse()
	raw, err := os.ReadFile(*inventory)
	if err != nil {
		return err
	}
	var modules []modelgen.Module
	if err := json.Unmarshal(raw, &modules); err != nil {
		return err
	}
	cfg := adminconfig.Load()
	if err := cfg.ValidateDatabase(); err != nil {
		return err
	}
	db, err := database.Open(context.Background(), cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		return err
	}
	pool, err := db.DB()
	if err != nil {
		return err
	}
	// Model outputs in the inventory are relative to the repository root.
	return errors.Join(modelgen.Generate(db, cfg.DBDriver, "..", modules, *selected), pool.Close())
}
