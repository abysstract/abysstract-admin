package module

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const backendImport = "github.com/abysstract/abysstract-admin/backend/"

var moduleName = regexp.MustCompile(`^[a-z][a-z0-9]{0,63}$`)
var exportedName = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
var jsName = regexp.MustCompile(`^[a-zA-Z_$][a-zA-Z0-9_$]*$`)

type selection struct {
	Modules []string `json:"modules"`
}
type entry struct {
	Package     string `json:"package"`
	Constructor string `json:"constructor"`
}
type frontend struct {
	Entry  string `json:"entry"`
	Export string `json:"export"`
	Import string `json:"import,omitempty"`
}
type generation struct {
	Name        string `json:"name"`
	TablePrefix string `json:"tablePrefix"`
	ModelOutput string `json:"modelOutput"`
	Prefix      string `json:"prefix"`
	GoOutput    string `json:"goOutput"`
	TSOutput    string `json:"tsOutput"`
	ProtoSource string `json:"protoSource"`
}
type resourceSpec struct {
	Package   string `json:"package"`
	Config    string `json:"config"`
	Resources string `json:"resources"`
}
type descriptor struct {
	Name       string        `json:"name"`
	Requires   []string      `json:"requires,omitempty"`
	Backend    entry         `json:"backend"`
	Frontend   frontend      `json:"frontend"`
	Generation *generation   `json:"generation,omitempty"`
	Resource   *resourceSpec `json:"resource,omitempty"`
	Seed       string        `json:"seed,omitempty"`
}

func readJSON(path string, dst any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func jsonBytes(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	return append(data, '\n'), err
}

// localPath validates paths at the module declaration boundary.
func localPath(root, path string) (string, error) {
	if path == "" || filepath.IsAbs(path) || strings.Contains(path, "\\") {
		return "", fmt.Errorf("expected repository-relative path: %q", path)
	}
	clean := filepath.Clean(path)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path outside repository: %q", path)
	}
	return filepath.Join(root, clean), nil
}

func readDescriptor(root, path string) (descriptor, error) {
	var d descriptor
	full, err := localPath(root, path)
	if err != nil {
		return d, err
	}
	if err := readJSON(full, &d); err != nil {
		return d, err
	}
	if !moduleName.MatchString(d.Name) || token.Lookup(d.Name).IsKeyword() {
		return d, fmt.Errorf("%s: invalid module name %q", path, d.Name)
	}
	if !exportedName.MatchString(d.Backend.Constructor) {
		return d, fmt.Errorf("%s: invalid constructor", path)
	}
	if !jsName.MatchString(d.Frontend.Export) {
		return d, fmt.Errorf("%s: invalid frontend export", path)
	}
	paths := []string{d.Backend.Package, d.Frontend.Entry}
	if d.Generation != nil {
		if err := validateGeneration(root, path, d.Name, *d.Generation); err != nil {
			return d, err
		}
		paths = append(paths, d.Generation.ProtoSource)
	}
	if d.Resource != nil {
		if !exportedName.MatchString(d.Resource.Config) || !exportedName.MatchString(d.Resource.Resources) {
			return d, fmt.Errorf("%s: invalid Resource exports", path)
		}
		paths = append(paths, d.Resource.Package)
	}
	if d.Seed != "" {
		paths = append(paths, d.Seed)
	}
	for _, p := range paths {
		f, err := localPath(root, p)
		if err != nil {
			return d, err
		}
		if _, err := os.Stat(f); err != nil {
			return d, err
		}
	}
	if !strings.HasPrefix(d.Backend.Package, "backend/") || (d.Resource != nil && !strings.HasPrefix(d.Resource.Package, "backend/")) {
		return d, fmt.Errorf("%s: Go packages must be under backend/", path)
	}
	return d, nil
}

func load(root string, selected selection) ([]descriptor, error) {
	defs := make([]descriptor, 0, len(selected.Modules))
	names := map[string]bool{}
	for _, path := range selected.Modules {
		d, err := readDescriptor(root, path)
		if err != nil {
			return nil, err
		}
		if names[d.Name] {
			return nil, fmt.Errorf("duplicate module %s", d.Name)
		}
		names[d.Name] = true
		defs = append(defs, d)
	}
	if !names["system"] {
		return nil, fmt.Errorf("system module is required")
	}
	return orderModules(defs, names)
}

func orderModules(defs []descriptor, names map[string]bool) ([]descriptor, error) {
	// Stable topological order also gives deterministic constructor order.
	var ordered []descriptor
	done := map[string]bool{}
	for len(ordered) < len(defs) {
		progress := false
		for _, d := range defs {
			if done[d.Name] {
				continue
			}
			ready := true
			for _, dep := range d.Requires {
				if !names[dep] {
					return nil, fmt.Errorf("module %s requires %s", d.Name, dep)
				}
				if !done[dep] {
					ready = false
				}
			}
			if ready {
				ordered = append(ordered, d)
				done[d.Name] = true
				progress = true
			}
		}
		if !progress {
			return nil, fmt.Errorf("module dependency cycle")
		}
	}
	owners := map[string]string{}
	for _, d := range ordered {
		if d.Generation == nil {
			continue
		}
		for _, path := range []string{d.Generation.ModelOutput, d.Generation.ProtoSource} {
			for other, owner := range owners {
				if owner != d.Name && (path == other || strings.HasPrefix(path, other+"/") || strings.HasPrefix(other, path+"/")) {
					return nil, fmt.Errorf("module %s path %s overlaps %s (%s)", d.Name, path, owner, other)
				}
			}
			owners[path] = d.Name
		}
	}
	return ordered, nil
}

func findRoot(start string) (string, error) {
	root, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(root, "backend/go.mod")); err == nil {
			return root, nil
		}
		parent := filepath.Dir(root)
		if parent == root {
			return "", fmt.Errorf("repository root not found from %s", start)
		}
		root = parent
	}
}

func validateGeneration(root, path, name string, g generation) error {
	if g.Name != name || g.TablePrefix == "" || !strings.HasSuffix(g.Prefix, "/") {
		return fmt.Errorf("%s: generation requires matching name, tablePrefix and protocol prefix", path)
	}
	for _, output := range []string{g.ModelOutput, g.GoOutput, g.TSOutput} {
		if _, err := localPath(root, output); err != nil {
			return err
		}
	}
	return nil
}
