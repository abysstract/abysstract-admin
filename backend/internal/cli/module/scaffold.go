package module

import (
	"bytes"
	"embed"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/abysstract/abysstract-admin/backend/moduleseed"
)

//go:embed templates/*
var scaffoldTemplates embed.FS

func scaffold(root, name string) (map[string][]byte, []string, error) {
	if !moduleName.MatchString(name) || token.Lookup(name).IsKeyword() || name == "system" {
		return nil, nil, fmt.Errorf("invalid business module name %q", name)
	}
	backend := "backend/internal/modules/" + name
	front := "frontend/apps/reference/src/modules/" + name
	for _, path := range []string{backend, front} {
		if _, err := os.Stat(filepath.Join(root, path)); err == nil {
			return nil, nil, fmt.Errorf("module directory already exists: %s", path)
		} else if !os.IsNotExist(err) {
			return nil, nil, err
		}
	}
	files, err := skeleton(name)
	if err != nil {
		return nil, nil, err
	}
	if err := checkModuleTablePrefix(root, descriptor{Name: name, Generation: &generation{Name: name, TablePrefix: name + "_"}}); err != nil {
		return nil, nil, err
	}
	return files, []string{
		"Created module conventions and resources/. Apply business table migrations, then run module resource " + name + " <table>.",
		"Generate: cd backend && go run ./cmd/admin module generate " + name,
		"Apply the module with: go run ./cmd/admin module update " + name,
	}, nil
}

func skeleton(name string) (map[string][]byte, error) {
	backend := "backend/internal/modules/" + name
	front := "frontend/apps/reference/src/modules/" + name
	d := descriptor{
		Name: name, Requires: []string{"system"}, Backend: entry{Package: backend, Constructor: "NewModule"}, Frontend: frontend{Entry: front + "/index.tsx", Export: name + "Module"},
		Generation: &generation{Name: name, TablePrefix: name + "_", ModelOutput: backend, Prefix: "company/" + name + "/v1/", GoOutput: backend + "/gen/proto", TSOutput: front + "/gen/proto", ProtoSource: backend + "/proto"},
		Resource:   &resourceSpec{Package: backend + "/definition", Config: "GeneratorConfig", Resources: "Resources"}, Seed: backend + "/seed.json",
	}
	files := map[string][]byte{}
	var err error
	files[backend+"/module.json"], err = jsonBytes(d)
	if err != nil {
		return nil, err
	}
	files[backend+"/resources/.gitkeep"] = nil
	files[d.Seed], err = jsonBytes(moduleseed.Spec{Module: name, Menus: []moduleseed.Menu{}})
	if err != nil {
		return nil, err
	}
	files[d.Generation.ProtoSource+"/company/"+name+"/v1/.gitkeep"] = nil
	src := fmt.Sprintf(`package %s
import (
"context"
"github.com/abysstract/abysstract-admin/backend/module"
)
func generatedModule() (module.Definition,error) {
 return module.Definition{Name:%q, Requires:[]string{"system"}, Create:func(context.Context, module.Runtime)(module.Instance,error){return module.Instance{},nil}},nil
}
`, name, name)
	files[backend+"/module.gen.go"], err = goSource(src)
	if err != nil {
		return nil, err
	}

	files[backend+"/module.go"], err = editableGo("package " + name + "\nimport \"github.com/abysstract/abysstract-admin/backend/module\"\nfunc NewModule() (module.Definition,error) { return generatedModule() }\n")
	if err != nil {
		return nil, err
	}
	files[front+"/index.tsx"] = []byte(fmt.Sprintf("import type { AdminFrontendModule } from \"@abysstract/admin-core\";\nimport { generatedPages } from \"./pages.gen\";\nexport const %sModule: AdminFrontendModule = { name: %q, pages: { ...generatedPages } };\n", name, name))
	files[front+"/pages.gen.ts"] = []byte(generatedHeader + "export const generatedPages = {};\n")
	files[backend+"/definition/resources.go"], err = editableGo("package definition\nimport \"github.com/abysstract/abysstract-admin/backend/resource/schema\"\nfunc Resources() []*schema.Resource { return generatedResources() }\n")
	if err != nil {
		return nil, err
	}
	files[backend+"/definition/resources.gen.go"], err = goSource("package definition\nimport \"github.com/abysstract/abysstract-admin/backend/resource/schema\"\nfunc generatedResources() []*schema.Resource { return nil }\n")
	if err != nil {
		return nil, err
	}
	view := crudView{Module: name, Backend: backend, Frontend: front, Import: backendImport + strings.TrimPrefix(backend, "backend/")}
	config, err := renderScaffold("config.go.tmpl", view)
	if err != nil {
		return nil, err
	}
	files[backend+"/definition/codegen.go"] = append([]byte(generatedHeader), config...)
	return files, nil
}

func editableGo(src string) ([]byte, error) {
	data, err := goSource(src)
	return bytes.TrimPrefix(data, []byte(generatedHeader)), err
}

func renderScaffold(name string, view crudView) ([]byte, error) {
	t, err := template.New(name).ParseFS(scaffoldTemplates, "templates/"+name)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := t.Execute(&out, view); err != nil {
		return nil, err
	}
	if strings.HasSuffix(name, ".go.tmpl") {
		return editableGo(out.String())
	}
	return out.Bytes(), nil
}
