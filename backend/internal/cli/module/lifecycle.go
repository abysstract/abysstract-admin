package module

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/abysstract/abysstract-admin/backend/adminconfig"
	"github.com/abysstract/abysstract-admin/backend/database"
	"github.com/abysstract/abysstract-admin/backend/moduleseed"
)

func lifecycleCommand(root *string, action string) *cobra.Command {
	var dry bool
	cmd := &cobra.Command{Use: action + " NAME", Args: cobra.ExactArgs(1), Short: map[string]string{"update": "Add or update a module and apply application migrations", "uninstall": "Remove registration and owned system records, retain business data", "status": "Show registration, migration and declaration state"}[action]}
	cmd.Flags().BoolVar(&dry, "dry-run", false, "Show planned changes without writing files or the database")
	cmd.RunE = func(c *cobra.Command, args []string) error {
		repo, err := findRoot(*root)
		if err != nil {
			return err
		}
		name := args[0]
		if !moduleName.MatchString(name) || name == "system" {
			return fmt.Errorf("invalid business module name %q", name)
		}
		if action == "status" {
			return moduleStatus(c, repo, name)
		}
		files, notes, err := prepareLifecycle(repo, action, name)
		if err != nil {
			return err
		}
		paths, err := writeFiles(repo, files, dry)
		if err != nil {
			return err
		}
		for _, p := range paths {
			fmt.Fprintln(c.OutOrStdout(), p)
		}
		for _, note := range notes {
			if !strings.HasPrefix(note, "Review the migration") {
				fmt.Fprintln(c.OutOrStdout(), note)
			}
		}
		if dry {
			fmt.Fprintln(c.OutOrStdout(), "Plan: apply pending application migrations; report database state after success.")
			return nil
		}
		migrate := exec.CommandContext(c.Context(), "go", "run", "./cmd/migrate")
		migrate.Dir = filepath.Join(repo, "backend")
		migrate.Stdout = c.OutOrStdout()
		migrate.Stderr = c.ErrOrStderr()
		if err := migrate.Run(); err != nil {
			return fmt.Errorf("%s %s: registration and migration files were saved; database migration failed: %w; inspect module status %s and rerun module %s %s after correcting the failure", action, name, err, name, action, name)
		}
		return moduleStatus(c, repo, name)
	}
	return cmd
}

func moduleStatus(c *cobra.Command, root, name string) error {
	var selected selection
	if err := readJSON(filepath.Join(root, "admin.modules.json"), &selected); err != nil {
		return err
	}
	registered := false
	path := "backend/internal/modules/" + name + "/module.json"
	for _, p := range selected.Modules {
		if p == path {
			registered = true
		}
	}
	fmt.Fprintf(c.OutOrStdout(), "Module %s: registered=%t\n", name, registered)
	registration, _, err := prepare(root, "sync", "")
	if err != nil {
		return err
	}
	drift, err := writeFiles(root, registration, true)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.OutOrStdout(), "Registration files current=%t\n", len(drift) == 0)
	if len(drift) > 0 {
		fmt.Fprintln(c.OutOrStdout(), "Run module sync to update: "+strings.Join(drift, ", "))
	}

	versions, err := reportApplicationMigrations(c, root)
	if err != nil {
		return err
	}
	paths, err := filepath.Glob(filepath.Join(root, "backend/migrations/modules/*_"+name+"_*.json"))
	if err != nil {
		return err
	}
	sort.Strings(paths)
	state := "no applied module snapshot"
	var latest *seedSnapshot
	pending := 0
	for _, path := range paths {
		stamp, _, _ := strings.Cut(filepath.Base(path), "_")
		version, err := strconv.ParseInt(stamp, 10, 64)
		if err != nil {
			return err
		}
		var snapshot seedSnapshot
		if err := readJSON(path, &snapshot); err != nil {
			return err
		}
		latest = &snapshot
		if versions[version] {
			state = snapshot.Action + " applied at " + stamp
		} else {
			pending++
			fmt.Fprintf(c.OutOrStdout(), "Pending module migration: %s\n", filepath.Base(path))
		}
	}
	fmt.Fprintf(c.OutOrStdout(), "Database: %s; pending module migrations=%d\n", state, pending)
	if registered {
		d, err := readDescriptor(root, path)
		if err != nil {
			return err
		}
		if d.Seed != "" {
			var spec moduleseed.Spec
			if err := readJSON(filepath.Join(root, d.Seed), &spec); err != nil {
				return err
			}
			if err := spec.Validate(); err != nil {
				return err
			}
			changed := latest == nil || latest.Action == "remove" || !equalSeed(latest.Seed, spec)
			fmt.Fprintf(c.OutOrStdout(), "Declaration changes awaiting update=%t\n", changed)
		}
	}
	return nil
}

func prepareLifecycle(repo, action, name string) (map[string][]byte, []string, error) {
	var selected selection
	if err := readJSON(filepath.Join(repo, "admin.modules.json"), &selected); err != nil {
		return nil, nil, err
	}
	path := "backend/internal/modules/" + name + "/module.json"
	registered := false
	for _, p := range selected.Modules {
		if p == path {
			registered = true
		}
	}
	operation := "sync"
	if action == "update" && !registered {
		operation = "add"
	}
	if action == "uninstall" && registered {
		operation = "remove"
	}
	files, notes, err := prepare(repo, operation, name)
	if err != nil {
		return nil, nil, err
	}
	if action == "update" && registered {
		d, err := readDescriptor(repo, path)
		if err != nil {
			return nil, nil, err
		}
		seeds, err := prepareSeed(repo, "upgrade", d)
		if err != nil {
			return nil, nil, err
		}
		for p, data := range seeds {
			files[p] = data
		}
		files["backend/migrations/go_registry.gen.go"], err = renderMigrations(repo, files)
		if err != nil {
			return nil, nil, err
		}
	}
	return files, notes, nil
}

func reportApplicationMigrations(c *cobra.Command, root string) (map[int64]bool, error) {
	cfg := adminconfig.Load()
	if err := cfg.ValidateDatabase(); err != nil {
		return nil, err
	}
	db, err := database.Open(context.Background(), cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		return nil, err
	}
	pool, err := db.DB()
	if err != nil {
		return nil, err
	}
	var applied []int64
	readErr := db.Raw("SELECT version_id FROM goose_db_version WHERE is_applied = ? AND id IN (SELECT MAX(id) FROM goose_db_version GROUP BY version_id)", true).Scan(&applied).Error
	if err := errors.Join(readErr, pool.Close()); err != nil {
		return nil, err
	}
	versions := map[int64]bool{}
	for _, version := range applied {
		versions[version] = true
	}

	applicationFiles, err := filepath.Glob(filepath.Join(root, "backend/migrations", cfg.DBDriver, "*"))
	if err != nil {
		return nil, err
	}
	pendingVersions := map[int64]bool{}
	for _, file := range applicationFiles {
		stamp, _, ok := strings.Cut(filepath.Base(file), "_")
		if !ok || len(stamp) != 14 {
			continue
		}
		version, err := strconv.ParseInt(stamp, 10, 64)
		if err != nil {
			return nil, err
		}
		if !versions[version] {
			pendingVersions[version] = true
		}
	}
	fmt.Fprintf(c.OutOrStdout(), "Pending application migrations=%d\n", len(pendingVersions))
	return versions, nil
}
