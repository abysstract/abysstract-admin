package module

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gorm.io/gorm/schema"

	"github.com/abysstract/abysstract-admin/backend/adminconfig"
	"github.com/abysstract/abysstract-admin/backend/database"
)

func resourceCommand(root *string) *cobra.Command {
	var name, title, updatedAt string
	var dry bool
	cmd := &cobra.Command{Use: "resource MODULE TABLE", Args: cobra.ExactArgs(2), Short: "Create an initial CRUD JSON from a migrated business table"}
	cmd.Flags().StringVar(&name, "name", "", "Resource name (default: table name without module prefix, in Go style)")
	cmd.Flags().StringVar(&title, "title", "", "Page and menu title (default: resource name)")
	cmd.Flags().StringVar(&updatedAt, "updated-at", "update_time", "Server-managed update timestamp column")
	cmd.Flags().BoolVar(&dry, "dry-run", false, "Show the initial JSON without writing files or changing the database")
	cmd.RunE = func(c *cobra.Command, args []string) error {
		repo, err := findRoot(*root)
		if err != nil {
			return err
		}
		module, table := args[0], args[1]
		if !moduleName.MatchString(module) || module == "system" {
			return fmt.Errorf("invalid business module name %q", module)
		}
		layout, err := loadLayout(repo)
		if err != nil {
			return err
		}
		backend, _ := layout.moduleDir(module)
		d, err := readDescriptor(repo, backend+"/module.json")
		if err != nil {
			return err
		}
		modelOutput := backend
		if d.Generation != nil {
			modelOutput = d.Generation.ModelOutput
		}
		if !columnName.MatchString(table) || !strings.HasPrefix(table, module+"_") {
			return fmt.Errorf("business table must use the %s_ prefix and snake_case: %q", module, table)
		}
		resourceName := name
		if !c.Flags().Changed("name") {
			resourceName = schema.NamingStrategy{}.SchemaName(strings.TrimPrefix(table, module+"_"))
		}
		resourceTitle := title
		if !c.Flags().Changed("title") {
			resourceTitle = resourceName
		}
		spec := crudSpec{Name: resourceName, Table: table, Title: resourceTitle, UpdatedAt: updatedAt}
		// Validate input before deriving a file path or opening the database.
		if err := validateCRUDHeader(module, spec); err != nil {
			return err
		}
		path := backend + "/resources/" + schema.NamingStrategy{}.ColumnName("", resourceName) + ".json"
		target := filepath.Join(repo, path)
		if _, err := os.Lstat(target); err == nil {
			return fmt.Errorf("CRUD configuration already exists: %s", path)
		} else if !os.IsNotExist(err) {
			return err
		}
		spec, err = inspectInitialResource(c.Context(), module, modelOutput, spec)
		if err != nil {
			return err
		}
		data, err := jsonBytes(spec)
		if err != nil {
			return err
		}
		if dry {
			fmt.Fprintf(c.OutOrStdout(), "%s\n%s", path, data)
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return fmt.Errorf("create CRUD configuration %s: %w", path, err)
		}
		_, writeErr := file.Write(data)
		if err := errors.Join(writeErr, file.Close()); err != nil {
			return err
		}
		fmt.Fprintln(c.OutOrStdout(), path)
		return nil
	}
	return cmd
}

func inspectInitialResource(ctx context.Context, module, modelOutput string, spec crudSpec) (crudSpec, error) {
	cfg := adminconfig.Load()
	if err := cfg.ValidateDatabase(); err != nil {
		return spec, err
	}
	db, err := database.Open(ctx, cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		return spec, err
	}
	pool, err := db.DB()
	if err != nil {
		return spec, err
	}
	meta, inspectErr := readCRUDModel(db, cfg.DBDriver, spec.Table, modelOutput)
	if inspectErr == nil {
		for _, f := range meta.Fields {
			if f.ColumnName != "id" && f.ColumnName != spec.UpdatedAt {
				spec.Fields = append(spec.Fields, crudField{Column: f.ColumnName, Label: f.ColumnName})
			}
		}
		inspectErr = validateCRUD(module, spec)
		if inspectErr == nil {
			_, inspectErr = inspectCRUDModel(module, spec, cfg.DBDriver, meta)
		}
	}
	if err := errors.Join(inspectErr, pool.Close()); err != nil {
		return spec, err
	}
	return spec, nil
}
