package module

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
)

// frameworkImport is the import prefix of the framework's public Go packages.
// Generated code imports module, resource, resourcegen and similar packages
// from here, and the application's own packages from appLayout.Import.
const frameworkImport = "github.com/abysstract/abysstract-admin/backend/"

// defaultFrontendRoot is the frontend source root of the bundled application.
const defaultFrontendRoot = "frontend/apps/reference/src"

// systemEntry selects the framework's system module in an application that
// does not contain the framework source.
const systemEntry = "system"

// appLayout describes the application the CLI runs in. The same commands work
// in the framework repository and in a separate application module.
type appLayout struct {
	// Import is the Go import path of backend/, ending in "/".
	Import string
	// FrontendRoot is the repository-relative frontend source directory that
	// holds modules.gen.ts and new modules.
	FrontendRoot string
	// Framework reports the framework repository, which also maintains the
	// repository-wide generation inventory (buf.yaml, codegen tasks).
	Framework bool
}

func loadLayout(root string) (appLayout, error) {
	var layout appLayout
	modulePath, base, err := goModule(root)
	if err != nil {
		return layout, err
	}
	layout.Import = modulePath + "/" + base
	layout.FrontendRoot = defaultFrontendRoot
	var selected selection
	if err := readJSON(filepath.Join(root, "admin.modules.json"), &selected); err != nil && !os.IsNotExist(err) {
		return layout, err
	}
	if selected.FrontendRoot != "" {
		if _, err := localPath(root, selected.FrontendRoot); err != nil {
			return layout, fmt.Errorf("admin.modules.json frontendRoot: %w", err)
		}
		layout.FrontendRoot = path.Clean(selected.FrontendRoot)
	}
	if _, err := os.Stat(filepath.Join(root, "buf.base.yaml")); err == nil {
		layout.Framework = true
	} else if !os.IsNotExist(err) {
		return layout, err
	}
	return layout, nil
}

// goModule returns the module path that owns backend/ and the import suffix of
// backend/ inside that module ("backend/" for a root go.mod, "" for
// backend/go.mod).
func goModule(root string) (string, string, error) {
	for _, candidate := range []struct{ file, base string }{{"go.mod", "backend/"}, {"backend/go.mod", ""}} {
		data, err := os.ReadFile(filepath.Join(root, candidate.file))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", "", err
		}
		modulePath := modfile.ModulePath(data)
		if modulePath == "" {
			return "", "", fmt.Errorf("%s: module path not found", candidate.file)
		}
		return modulePath, candidate.base, nil
	}
	return "", "", fmt.Errorf("go.mod not found in %s or %s/backend", root, root)
}

// importPath maps a repository-relative directory under backend/ to its Go
// import path.
func (l appLayout) importPath(dir string) string {
	return l.Import + strings.TrimPrefix(path.Clean(dir), "backend/")
}

// moduleDir is the conventional directory of a new business module.
func (l appLayout) moduleDir(name string) (backend, front string) {
	return "backend/internal/modules/" + name, l.FrontendRoot + "/modules/" + name
}

// systemDescriptor is the framework system module as seen by an application:
// its code, protocol and pages are already generated in the framework.
func systemDescriptor() descriptor {
	return descriptor{
		Name:     "system",
		Backend:  entry{Constructor: "NewModule"},
		Frontend: frontend{Export: "coreSystemModule", Import: "@abysstract/admin-core/system"},
		builtin:  frameworkImport + "system",
	}
}

// relativeImport returns the TypeScript import specifier of target as seen from
// a file in dir.
func relativeImport(dir, target string) (string, error) {
	rel, err := filepath.Rel(filepath.FromSlash(dir), filepath.FromSlash(target))
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, ".") {
		rel = "./" + rel
	}
	return rel, nil
}
