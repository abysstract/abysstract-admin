// Package module implements the module development command group.
package module

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	var root string
	cmd := &cobra.Command{Use: "module", Short: "Develop and manage application modules"}
	cmd.PersistentFlags().StringVar(&root, "root", ".", "Repository root or a directory within it")
	for _, action := range []string{"new", "sync", "check"} {
		sub := &cobra.Command{Use: action, Short: map[string]string{"new": "Create module directories and base files", "sync": "Synchronize runtime registration and development inventory", "check": "Check generated registration without writing"}[action]}
		if action == "new" {
			sub.Use += " NAME"
			sub.Args = cobra.ExactArgs(1)
		} else {
			sub.Args = cobra.NoArgs
		}
		var dry bool
		sub.Flags().BoolVar(&dry, "dry-run", false, "Show changes without writing files or changing the database")
		sub.RunE = func(c *cobra.Command, args []string) error {
			repo, err := findRoot(root)
			if err != nil {
				return err
			}
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			files, notes, err := prepare(repo, action, name)
			if err != nil {
				return err
			}
			paths, err := writeFiles(repo, files, dry || action == "check")
			if err != nil {
				return err
			}
			for _, path := range paths {
				fmt.Fprintln(c.OutOrStdout(), path)
			}
			for _, note := range notes {
				fmt.Fprintln(c.OutOrStdout(), note)
			}
			if action == "check" && len(paths) > 0 {
				return fmt.Errorf("module registration differs; run admin module sync")
			}
			if len(paths) == 0 {
				fmt.Fprintln(c.OutOrStdout(), "Module registration is current.")
			}
			return nil
		}
		cmd.AddCommand(sub)
	}
	for _, action := range []string{"update", "uninstall", "status"} {
		cmd.AddCommand(lifecycleCommand(&root, action))
	}
	cmd.AddCommand(generateCommand(&root), resourceCommand(&root))
	return cmd
}

func prepare(root, action, name string) (map[string][]byte, []string, error) {
	if action == "new" {
		return scaffold(root, name)
	}
	var selected selection
	if err := readJSON(filepath.Join(root, "admin.modules.json"), &selected); err != nil {
		return nil, nil, err
	}
	var err error
	var changed descriptor
	var notes []string
	if action == "add" || action == "remove" {
		selected, changed, notes, err = changeSelection(root, selected, action, name)
		if err != nil {
			return nil, nil, err
		}
	}
	var seedFiles map[string][]byte

	defs, err := load(root, selected)
	if err != nil {
		return nil, nil, err
	}
	if action == "remove" {
		if err := checkReferences(root, defs, name); err != nil {
			return nil, nil, err
		}
	}
	if action == "add" || action == "remove" {
		seedFiles, err = prepareSeed(root, action, changed)
		if err != nil {
			return nil, nil, err
		}
		seedNotes, err := describeSeedFiles(seedFiles)
		if err != nil {
			return nil, nil, err
		}
		notes = append(notes, seedNotes...)
	}

	files, err := render(root, defs)
	if err != nil {
		return nil, nil, err
	}
	if action == "add" || action == "remove" {
		files["admin.modules.json"], err = jsonBytes(selected)
		if err != nil {
			return nil, nil, err
		}
		for path, data := range seedFiles {
			files[path] = data
		}
		files["backend/migrations/go_registry.gen.go"], err = renderMigrations(root, files)
		if err != nil {
			return nil, nil, err
		}
	}
	return files, notes, nil
}

func checkReferences(root string, defs []descriptor, removed string) error {
	needles := []string{"internal/modules/" + removed + "\"", "internal/modules/" + removed + "/", "modules/" + removed + "/"}
	removedPath := "backend/internal/modules/" + removed + "/module.json"
	if _, err := os.Stat(filepath.Join(root, removedPath)); err == nil {
		d, err := readDescriptor(root, removedPath)
		if err != nil {
			return err
		}
		if d.Generation != nil {
			needles = append(needles, d.Generation.Prefix)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	for _, d := range defs {
		dirs := []string{d.Backend.Package, filepath.Dir(d.Frontend.Entry)}
		if d.Generation != nil {
			dirs = append(dirs, d.Generation.ProtoSource)
		}
		for _, dir := range dirs {
			err := filepath.WalkDir(filepath.Join(root, dir), func(path string, e os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if e.IsDir() {
					return nil
				}
				ext := filepath.Ext(path)
				if ext != ".go" && ext != ".ts" && ext != ".tsx" && ext != ".proto" {
					return nil
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				for _, needle := range needles {
					if strings.Contains(string(data), needle) {
						return fmt.Errorf("module %s references removed module %s in %s", d.Name, removed, path)
					}
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func changeSelection(root string, selected selection, action, name string) (selection, descriptor, []string, error) {
	var changed descriptor
	var notes []string
	if !moduleName.MatchString(name) {
		return selected, changed, notes, fmt.Errorf("invalid module name %q", name)
	}
	if name == "system" {
		return selected, changed, notes, fmt.Errorf("system is the base module")
	}
	path := "backend/internal/modules/" + name + "/module.json"
	index := -1
	for i, p := range selected.Modules {
		if p == path {
			index = i
		}
	}
	if action == "add" {
		if index >= 0 {
			return selected, changed, notes, fmt.Errorf("module %s is already selected", name)
		}
		var err error
		changed, err = readDescriptor(root, path)
		if err != nil {
			return selected, changed, notes, fmt.Errorf("read module declaration (create new modules with 'module new'): %w", err)
		}
		if changed.Name != name {
			return selected, changed, notes, fmt.Errorf("module declaration name %s differs from %s", changed.Name, name)
		}
		selected.Modules = append(selected.Modules, path)
	} else {
		if index < 0 {
			return selected, changed, notes, fmt.Errorf("module %s is not selected", name)
		}
		// Removal can recover from deleted source using the conventional module location.
		changed = descriptor{Name: name}
		selected.Modules = append(selected.Modules[:index], selected.Modules[index+1:]...)
		notes = append(notes, "After applying the cleanup migration, source directories may be removed: backend/internal/modules/"+name+", frontend/apps/reference/src/modules/"+name)
	}
	return selected, changed, notes, nil
}

func describeSeedFiles(seedFiles map[string][]byte) ([]string, error) {
	var notes []string
	if len(seedFiles) == 0 {
		return notes, nil
	}

	for path, data := range seedFiles {
		if !strings.HasPrefix(path, "backend/migrations/modules/") {
			continue
		}
		var snapshot seedSnapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			return nil, err
		}
		notes = append(notes, fmt.Sprintf("Seed %s %s: %d menus, %d example roles. Business tables and data are retained.", snapshot.Action, snapshot.Seed.Module, len(snapshot.Seed.Menus), len(snapshot.Seed.Roles)))
		for _, menu := range snapshot.Seed.Menus {
			notes = append(notes, fmt.Sprintf("  menu %s: %s [%s]", menu.Key, menu.Name, menu.Permission))
		}
		for _, role := range snapshot.Seed.Roles {
			notes = append(notes, "  example role "+role.Key)
		}
	}
	notes = append(notes, "Review the migration diff, then run: cd backend && go run ./cmd/migrate")
	return notes, nil
}
