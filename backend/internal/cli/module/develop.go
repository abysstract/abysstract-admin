package module

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/adminconfig"
	"github.com/abysstract/abysstract-admin/backend/database"
	"github.com/abysstract/abysstract-admin/backend/moduleseed"
)

type developmentState struct {
	Protocols  map[string]protocolFields  `json:"protocols"`
	Files      map[string]string          `json:"files"`
	Seed       moduleseed.Spec            `json:"seed"`
	Operations map[string][]crudOperation `json:"operations,omitempty"`
}

func generateCommand(root *string) *cobra.Command {
	var dry bool
	cmd := &cobra.Command{Use: "generate NAME", Args: cobra.ExactArgs(1), Short: "Generate models, CRUD contracts and implementations from migrated tables"}
	cmd.Flags().BoolVar(&dry, "dry-run", false, "Validate inputs and show planned files without writing")
	cmd.RunE = func(c *cobra.Command, args []string) error {
		repo, err := findRoot(*root)
		if err != nil {
			return err
		}
		files, err := prepareDevelopment(repo, args[0])
		if err != nil {
			return err
		}
		paths, err := writeFiles(repo, files, dry)
		if err != nil {
			return err
		}
		for _, path := range paths {
			fmt.Fprintln(c.OutOrStdout(), path)
		}
		if dry {
			fmt.Fprintln(c.OutOrStdout(), "Plan: generate Model/query, protocol types and Resource implementations for this module.")
			return nil
		}
		registration, _, err := prepare(repo, "sync", "")
		if err != nil {
			return err
		}
		if _, err := writeFiles(repo, registration, false); err != nil {
			return err
		}
		temp, err := os.MkdirTemp(filepath.Join(repo, "backend"), ".module-generate-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(temp)
		steps, err := developmentGenerationSteps(repo, args[0], temp)
		if err != nil {
			return err
		}

		for _, step := range steps {
			run := exec.CommandContext(c.Context(), step[0], step[1:]...)
			run.Dir = filepath.Join(repo, "backend")
			run.Stdout = c.OutOrStdout()
			run.Stderr = c.ErrOrStderr()
			if err := run.Run(); err != nil {
				return fmt.Errorf("generation failed at %s: %w; correct the error and rerun module generate %s", strings.Join(step, " "), err, args[0])
			}
		}
		fmt.Fprintf(c.OutOrStdout(), "Generation complete. Run module update %s when ready to use these changes in the application.\n", args[0])
		return nil
	}
	return cmd
}

func prepareDevelopment(root, name string) (map[string][]byte, error) {
	if !moduleName.MatchString(name) || name == "system" {
		return nil, fmt.Errorf("invalid business module name %q", name)
	}
	backend := "backend/internal/modules/" + name
	front := "frontend/apps/reference/src/modules/" + name
	d, err := readDescriptor(root, backend+"/module.json")
	if err != nil {
		return nil, err
	}
	if err := checkModuleTablePrefix(root, d); err != nil {
		return nil, err
	}
	paths, err := filepath.Glob(filepath.Join(root, backend, "resources/*.json"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		if _, err := os.Stat(filepath.Join(root, backend, "resources.lock.json")); err == nil {
			return nil, fmt.Errorf("resource declarations were removed; restore them before generation")
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		return map[string][]byte{}, nil
	}
	statePath := backend + "/resources.lock.json"
	previous := developmentState{Protocols: map[string]protocolFields{}, Files: map[string]string{}}
	if _, err := os.Stat(filepath.Join(root, statePath)); err == nil {
		if err := readJSON(filepath.Join(root, statePath), &previous); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
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
	views, next, inspectErr := inspectDevelopmentResources(root, name, backend, paths, previous, db, cfg.DBDriver)
	if err := errors.Join(inspectErr, pool.Close()); err != nil {
		return nil, err
	}
	return renderDevelopment(root, name, backend, front, d, views, previous, next)
}

func renderDevelopment(root, name, backend, front string, d descriptor, views []crudView, previous, next developmentState) (map[string][]byte, error) {
	var err error
	statePath := backend + "/resources.lock.json"

	files := map[string][]byte{}
	editable := map[string][]byte{}
	var resources, modules, pages strings.Builder
	resources.WriteString("package definition\nimport \"github.com/abysstract/abysstract-admin/backend/resource/schema\"\nfunc generatedResources() []*schema.Resource { return []*schema.Resource{")
	modules.WriteString("package " + name + "\nimport (\n\"context\"\n\"github.com/abysstract/abysstract-admin/backend/module\"\n\"github.com/abysstract/abysstract-admin/backend/permission\"\n)\nfunc generatedModule() (module.Definition,error) {\nvar defs []module.Definition\n")
	pages.WriteString(generatedHeader + "import { lazy } from \"react\";\nexport const generatedPages = {\n")
	seed := moduleseed.Spec{Module: name, Menus: []moduleseed.Menu{{Key: name + ".root", Name: name, Path: name, Type: "M", Order: 60}}}
	for _, view := range views {
		var retired []string
		for column, field := range next.Protocols[view.Name] {
			if field.Retired {
				retired = append(retired, column)
			}
		}
		sort.Strings(retired)
		for _, column := range retired {
			f := next.Protocols[view.Name][column]
			view.Reserved = append(view.Reserved, retiredField{Name: column, Entity: f.Entity, Create: f.Create, Managed: f.Managed})
		}
		rendered, resourceMenus, err := renderCRUD(view)
		if err != nil {
			return nil, err
		}
		for _, path := range []string{backend + "/" + view.File + ".gen.go", backend + "/definition/" + view.File + ".gen.go", backend + "/proto/company/" + name + "/v1/" + view.File + ".proto"} {
			files[path] = append([]byte(generatedHeader), rendered[path]...)
		}
		resources.WriteString(view.Name + "Resource,\n")
		editable[backend+"/"+view.File+".go"], err = editableGo(fmt.Sprintf("package %s\nimport (gen %q; %q)\nfunc configure%s(rt module.Runtime, deps gen.%sControllerDeps) gen.%sControllerDeps { return deps }\n", name, view.Import+"/gen/resource", backendImport+"module", view.Name, view.Name, view.Name))
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&modules, "d%s,err := generated%sModule(); if err != nil { return module.Definition{},err }; defs = append(defs,d%s)\n", view.Name, view.Name, view.Name)
		fmt.Fprintf(&pages, "%q: lazy(() => import(%q)),\n", "biz/"+name+"/"+view.File, "./pages/"+view.Name+"List")
		editable[front+"/pages/"+view.Name+"List.tsx"] = rendered[front+"/pages/"+view.Name+"List.tsx"]
		seed.Menus = append(seed.Menus, resourceMenus...)
	}
	resources.WriteString("} }\n")
	files[backend+"/definition/resources.gen.go"], err = goSource(resources.String())
	if err != nil {
		return nil, err
	}
	modules.WriteString(`
 result := module.Definition{Name: "` + name + `", Requires: []string{"system"}}
 for _,d := range defs { result.Services = append(result.Services,d.Services...) }
 result.Permissions = func(r permission.Registrar) error { for _,d := range defs { if err := d.Permissions(r); err != nil { return err } }; return nil }
 result.Create = func(ctx context.Context,rt module.Runtime) (module.Instance,error) {
  result := module.Instance{}
  for _,d := range defs { instance,err := d.Create(ctx,rt); if err != nil { return module.Instance{},err }; result.Services = append(result.Services,instance.Services...) }
  return result,nil
 }
 return result,nil
}
`)
	files[backend+"/module.gen.go"], err = goSource(modules.String())
	if err != nil {
		return nil, err
	}
	pages.WriteString("};\n")
	files[front+"/pages.gen.ts"] = []byte(pages.String())

	configView := views[0]
	configView.Resources = views
	mapped := map[string]bool{}
	for _, view := range views {
		for _, source := range view.CustomImports {
			if !mapped[source] {
				configView.CustomProtoMappings = append(configView.CustomProtoMappings, protoMapping{Source: source, Target: "../proto/" + strings.TrimSuffix(source, ".proto") + "_pb"})
				mapped[source] = true
			}
		}
	}

	cfgSource, err := renderScaffold("config.go.tmpl", configView)
	if err != nil {
		return nil, err
	}
	files[backend+"/definition/codegen.go"] = append([]byte(generatedHeader), cfgSource...)
	next.Seed = seed
	var current moduleseed.Spec
	if err := readJSON(filepath.Join(root, d.Seed), &current); err != nil {
		return nil, err
	}
	if err := current.Validate(); err != nil {
		return nil, err
	}
	merged, err := mergeDevelopmentMenus(current, previous.Seed, seed)
	if err != nil {
		return nil, err
	}
	files[d.Seed], err = jsonBytes(merged)
	if err != nil {
		return nil, err
	}
	if err := preserveDevelopmentFiles(root, name, d.Seed, files, editable, previous, &next); err != nil {
		return nil, err
	}
	files[statePath], err = jsonBytes(next)
	return files, err
}

func developmentGenerationSteps(repo, name, temp string) ([][]string, error) {
	// Include the explicit target only in this invocation's generation inputs.
	var selected selection
	if err := readJSON(filepath.Join(repo, "admin.modules.json"), &selected); err != nil {
		return nil, err
	}
	defs, err := load(repo, selected)
	if err != nil {
		return nil, err
	}
	target, err := readDescriptor(repo, "backend/internal/modules/"+name+"/module.json")
	if err != nil {
		return nil, err
	}
	if target.Generation == nil || target.Resource == nil {
		return nil, fmt.Errorf("module %s requires generation and resource configuration", target.Name)
	}
	found := false
	for _, d := range defs {
		if d.Name == target.Name {
			found = true
		}
	}
	if !found {
		defs = append(defs, target)
	}
	if err := checkTablePrefixes(defs); err != nil {
		return nil, err
	}
	inputs, err := render(repo, defs)
	if err != nil {
		return nil, err
	}
	inventory := filepath.Join(temp, "modules.json")
	config := filepath.Join(temp, "buf.yaml")
	if err := os.WriteFile(inventory, inputs["backend/internal/codegen/modules.json"], 0600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(config, inputs["buf.yaml"], 0600); err != nil {
		return nil, err
	}
	runner, err := goSource(fmt.Sprintf(`package main
import (
 "fmt"
 "os"
 definition %q
 %q
)
func main() {
 if err := resourcegen.Generate(definition.%s(%q), definition.%s()); err != nil {
  fmt.Fprintln(os.Stderr, err)
  os.Exit(1)
 }
}
`, importPath(target.Resource.Package), backendImport+"resourcegen", target.Resource.Config, repo, target.Resource.Resources))
	if err != nil {
		return nil, err
	}
	runnerPath := filepath.Join(temp, "main.go")
	if err := os.WriteFile(runnerPath, runner, 0600); err != nil {
		return nil, err
	}
	return [][]string{{"go", "run", "./cmd/modelgen", "--inventory", inventory, "--module", name}, {"node", "../frontend/tools/generate-proto.mjs", "--inventory", inventory, "--config", config}, {"go", "run", runnerPath}}, nil
}

func inspectDevelopmentResources(root, name, backend string, paths []string, previous developmentState, db *gorm.DB, driver string) ([]crudView, developmentState, error) {
	var views []crudView
	next := developmentState{Protocols: map[string]protocolFields{}, Files: map[string]string{}, Operations: map[string][]crudOperation{}}

	for _, path := range paths {
		var spec crudSpec
		if err := readJSON(path, &spec); err != nil {
			return nil, next, err
		}
		if err := validateCRUD(name, spec); err != nil {
			return nil, next, err
		}
		if _, exists := next.Protocols[spec.Name]; exists {
			return nil, next, fmt.Errorf("duplicate resource %s", spec.Name)
		}
		view, err := inspectCRUD(db, driver, name, spec)
		if err != nil {
			return nil, next, err
		}
		fields, err := assignProtocol(&view, previous.Protocols[spec.Name])
		if err != nil {
			return nil, next, err
		}

		if err := validateOperationChanges(previous.Operations[spec.Name], spec.Operations); err != nil {
			return nil, next, err
		}
		for _, source := range view.CustomImports {
			if source == "company/"+name+"/v1/"+view.File+".proto" {
				return nil, next, fmt.Errorf("resource %s imports its own protocol", spec.Name)
			}
			if _, err := os.Stat(filepath.Join(root, backend, "proto", source)); err != nil {
				return nil, next, err
			}
		}
		next.Operations[spec.Name] = spec.Operations
		next.Protocols[spec.Name] = fields
		views = append(views, view)
	}
	for resource := range previous.Protocols {
		if _, exists := next.Protocols[resource]; !exists {
			return nil, next, fmt.Errorf("resource %s was removed; retire its public service through an explicit versioned API change", resource)
		}
	}
	return views, next, nil
}

func validateOperationChanges(previous, current []crudOperation) error {
	for _, old := range previous {
		found := false
		for _, op := range current {
			if op.Method == old.Method {
				found = true
				if op.Request != old.Request || op.Response != old.Response {
					return fmt.Errorf("custom operation %s changes its public signature; introduce a versioned contract", op.Method)
				}
			}
		}
		if !found {
			return fmt.Errorf("custom operation %s was removed; introduce a versioned contract", old.Method)
		}
	}
	return nil
}

func mergeDevelopmentMenus(current, previous, seed moduleseed.Spec) (moduleseed.Spec, error) {
	oldMenus := map[string]moduleseed.Menu{}
	for _, m := range previous.Menus {
		oldMenus[m.Key] = m
	}
	newMenus := map[string]moduleseed.Menu{}
	for _, m := range seed.Menus {
		newMenus[m.Key] = m
	}
	merged := current
	merged.Menus = nil
	for _, m := range current.Menus {
		old, managed := oldMenus[m.Key]
		fresh, exists := newMenus[m.Key]
		if managed && !exists {
			continue
		}
		if managed && m == old {
			m = fresh
		}
		merged.Menus = append(merged.Menus, m)
		delete(newMenus, m.Key)
	}
	for _, m := range seed.Menus {
		if _, exists := newMenus[m.Key]; exists {
			if _, previouslyGenerated := oldMenus[m.Key]; !previouslyGenerated {
				merged.Menus = append(merged.Menus, m)
			}
		}
	}
	if err := merged.Validate(); err != nil {
		return moduleseed.Spec{}, err
	}
	return merged, nil
}

func preserveDevelopmentFiles(root, name, seedPath string, files, editable map[string][]byte, previous developmentState, next *developmentState) error {
	initial, err := skeleton(name)
	if err != nil {
		return err
	}
	for path, data := range files {
		if path == seedPath {
			continue
		}
		old, err := os.ReadFile(filepath.Join(root, path))
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil {
			sum := sha256.Sum256(old)
			initialData, skeletonOwned := initial[path]
			if previous.Files[path] != hex.EncodeToString(sum[:]) && (!skeletonOwned || !bytes.Equal(old, initialData)) {
				return fmt.Errorf("generated destination %s has edits; move business changes to its handwritten extension", path)
			}
		}
		sum := sha256.Sum256(data)
		next.Files[path] = hex.EncodeToString(sum[:])
	}
	for path, data := range editable {
		old, err := os.ReadFile(filepath.Join(root, path))
		initialData, skeletonOwned := initial[path]
		if os.IsNotExist(err) || err == nil && skeletonOwned && bytes.Equal(old, initialData) {
			files[path] = data
		} else if err != nil {
			return err
		}
	}
	return nil
}
