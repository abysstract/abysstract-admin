package resourcegen

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"

	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

// Generate validates, renders, and writes Go/TS resource code for the explicit
// resource list. An empty list only cleans this invocation's managed files.
func Generate(config Config, resources []*schema.Resource) error {
	return generate(config, resources, osFS{})
}

type generator struct {
	cfg     Config
	fs      fileSystem
	version string
}

func generate(cfg Config, resources []*schema.Resource, fsys fileSystem) error {
	if err := validateConfig(cfg); err != nil {
		return err
	}
	cfg.GeneratorVersion, cfg.SourceIdentity = resolveGeneratorIdentity(cfg)
	root, err := filepath.EvalSymlinks(cfg.RootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("resourcegen: RootDir does not exist")
		}
		return fmt.Errorf("resourcegen: resolve RootDir: %w", err)
	}
	cfg.RootDir = root
	goDir, err := checkOutputLocation(fsys, cfg.RootDir, cfg.GoOutputDir, "Go")
	if err != nil {
		return err
	}
	tsDir, err := checkOutputLocation(fsys, cfg.RootDir, cfg.TSOutputDir, "TS")
	if err != nil {
		return err
	}
	g := &generator{cfg: cfg, fs: fsys, version: cfg.GeneratorVersion}
	plans, err := schema.BuildRegistry(resources)
	if err != nil {
		return err
	}
	if err := checkProtoMappings(cfg, plans); err != nil {
		return err
	}
	goOut, tsOut, err := g.renderAll(plans)
	if err != nil {
		return err
	}
	formatted, err := formatGoOutFile(formatGoFile(cfg, g.version))
	if err != nil {
		return err
	}
	goOut = append([]outFile{formatted}, goOut...)
	tsOut = append([]outFile{formatTSFile(cfg, g.version)}, tsOut...)
	if err := g.checkOutputs(goDir, "go", ".gen.go", goOut); err != nil {
		return err
	}
	if err := g.checkOutputs(tsDir, "ts", ".gen.ts", tsOut); err != nil {
		return err
	}
	if err := fsys.MkdirAll(goDir, 0o755); err != nil {
		return err
	}
	if err := fsys.MkdirAll(tsDir, 0o755); err != nil {
		return err
	}
	if err := g.writeFiles(goDir, goOut); err != nil {
		return err
	}
	if err := g.writeFiles(tsDir, tsOut); err != nil {
		return err
	}
	if err := g.reconcile(goDir, "go", ".gen.go", fileNames(goOut)); err != nil {
		return err
	}
	if err := g.reconcile(tsDir, "ts", ".gen.ts", fileNames(tsOut)); err != nil {
		return err
	}
	return nil
}

func (g *generator) renderAll(plans []*schema.Plan) ([]outFile, []outFile, error) {
	var goOut, tsOut []outFile
	for _, plan := range plans {
		files, err := g.renderGo(plan)
		if err != nil {
			return nil, nil, err
		}
		goOut = append(goOut, files...)
		tn, tc, err := g.renderTS(plan)
		if err != nil {
			return nil, nil, err
		}
		tsOut = append(tsOut, outFile{tn, tc})
		operationFiles, err := g.renderTSOperations(plan)
		if err != nil {
			return nil, nil, err
		}
		tsOut = append(tsOut, operationFiles...)
	}
	serviceAdapters, err := g.renderServiceAdapters(plans)
	if err != nil {
		return nil, nil, err
	}
	goOut = append(goOut, serviceAdapters...)
	registry, err := g.renderTSOperationsRegistry(plans)
	if err != nil {
		return nil, nil, err
	}
	if registry.name != "" {
		tsOut = append(tsOut, registry)
	}
	for _, plan := range plans {
		relations, err := g.renderTSRelations(plan, plans)
		if err != nil {
			return nil, nil, err
		}
		tsOut = append(tsOut, relations...)
	}
	return goOut, tsOut, nil
}

func (g *generator) checkOutputs(dir, kind, suffix string, files []outFile) error {
	seen := map[string]bool{}
	for _, file := range files {
		if !safeManagedName(file.name, suffix) {
			return fmt.Errorf("resourcegen: illegal output name %q", file.name)
		}
		if seen[file.name] {
			return fmt.Errorf("resourcegen: duplicate output name %q", file.name)
		}
		seen[file.name] = true
		if err := inspectExistingFile(g.fs, dir, file.name); err != nil {
			return err
		}
	}
	previous, err := readManifest(g.fs, dir, kind)
	if err != nil {
		return err
	}
	for _, name := range previous {
		if !safeManagedName(name, suffix) {
			return fmt.Errorf("resourcegen: illegal manifest item %q", name)
		}
		if seen[name] {
			continue
		}
		if err := inspectExistingFile(g.fs, dir, name); err != nil {
			return fmt.Errorf("resourcegen: cannot clean %s: %w", name, err)
		}
	}
	return nil
}

func (g *generator) writeFiles(dir string, files []outFile) error {
	for _, file := range files {
		path := filepath.Join(dir, file.name)
		if err := g.fs.WriteFile(path, file.content, 0o644); err != nil {
			return fmt.Errorf("resourcegen: write %s: %w", file.name, err)
		}
	}
	return nil
}

func (g *generator) reconcile(dir, kind, suffix string, current []string) error {
	previous, err := readManifest(g.fs, dir, kind)
	if err != nil {
		return err
	}
	keep := map[string]bool{}
	for _, name := range current {
		keep[name] = true
	}
	for _, name := range previous {
		if keep[name] {
			continue
		}
		if !safeManagedName(name, suffix) {
			return fmt.Errorf("resourcegen: illegal manifest item %q", name)
		}
		if err := inspectExistingFile(g.fs, dir, name); err != nil {
			return fmt.Errorf("resourcegen: cannot clean %s: %w", name, err)
		}
		path := filepath.Join(dir, name)
		if err := g.fs.Remove(path); err != nil && !isNotExist(err) {
			return fmt.Errorf("resourcegen: remove %s: %w", name, err)
		}
	}
	names := append([]string{}, current...)
	sort.Strings(names)
	manifestPath := filepath.Join(dir, ".adminresource-"+kind+".manifest")
	if err := g.fs.WriteFile(manifestPath, manifestBytes(names), 0o644); err != nil {
		return fmt.Errorf("resourcegen: write %s manifest: %w", kind, err)
	}
	return nil
}

func fileNames(files []outFile) []string {
	names := make([]string, 0, len(files))
	for _, file := range files {
		names = append(names, file.name)
	}
	return names
}

func formatGoOutFile(file outFile) (outFile, error) {
	src, err := format.Source(file.content)
	if err != nil {
		return outFile{}, fmt.Errorf("gofmt %s: %w\n---\n%s", file.name, err, file.content)
	}
	file.content = src
	return file, nil
}
