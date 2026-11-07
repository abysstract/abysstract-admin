package module

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	frameworkModule = "github.com/abysstract/abysstract-admin"
	systemProtoPath = "abysstract/admin/v1/"
)

// protoPlugins mirrors proto/buf.gen.yaml; a test keeps them in step.
var protoPlugins = []struct{ remote, out, opt string }{
	{"buf.build/protocolbuffers/go:v1.36.12", "go", "paths=source_relative"},
	{"buf.build/connectrpc/go:v2.0.2", "go", "paths=source_relative"},
	{"buf.build/bufbuild/es:v2.14.0", "ts", "target=ts"},
}

// generateProtocol runs buf for one module and writes its Go and TypeScript
// protocol output. Framework protocol sources come from the framework module
// the application depends on, so applications need only buf on PATH.
func generateProtocol(ctx context.Context, root string, layout appLayout, defs []descriptor, target string) error {
	core, err := frameworkProtoDir(ctx, root, layout)
	if err != nil {
		return err
	}
	temp, err := os.MkdirTemp("", "admin-proto-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)

	sources, tasks, err := stageProtos(root, core, defs, temp)
	if err != nil {
		return err
	}
	task := findGeneration(tasks, target)
	if task == nil {
		return fmt.Errorf("module %s has no generation configuration", target)
	}
	if err := writeBufConfig(temp, layout, sources, tasks); err != nil {
		return err
	}
	if err := runBuf(ctx, temp); err != nil {
		return err
	}
	goOutput, tsOutput, err := collectProtocol(temp, tasks, *task)
	if err != nil {
		return err
	}
	return writeProtocol(root, *task, goOutput, tsOutput)
}

// stageProtos copies the framework and every generating module's Proto
// sources into the buf workspace, returning its module dirs and generations.
func stageProtos(root, core string, defs []descriptor, temp string) ([]string, []generation, error) {
	sources := []string{"src/core"}
	if err := copyProtos(core, filepath.Join(temp, "src/core")); err != nil {
		return nil, nil, err
	}
	var tasks []generation
	for i, d := range defs {
		if d.builtin != "" || d.Generation == nil {
			continue
		}
		tasks = append(tasks, *d.Generation)
		source := filepath.Join(root, filepath.FromSlash(d.Generation.ProtoSource))
		if same, err := sameDir(source, core); err != nil {
			return nil, nil, err
		} else if same {
			continue
		}
		dir := fmt.Sprintf("src/m%d", i)
		if err := copyProtos(source, filepath.Join(temp, dir)); err != nil {
			return nil, nil, err
		}
		if _, err := os.Stat(filepath.Join(temp, dir)); err == nil {
			sources = append(sources, dir)
		}
	}
	return sources, tasks, nil
}

func findGeneration(tasks []generation, name string) *generation {
	var task *generation
	for i := range tasks {
		if tasks[i].Name == name {
			task = &tasks[i]
		}
	}
	return task
}

// writeBufConfig writes the workspace and a template that places every
// module's Go package under the application import path.
func writeBufConfig(temp string, layout appLayout, sources []string, tasks []generation) error {
	var workspace strings.Builder
	workspace.WriteString("version: v2\nmodules:\n")
	for _, dir := range sources {
		fmt.Fprintf(&workspace, "  - path: %s\n", dir)
	}
	var template strings.Builder
	template.WriteString("version: v2\nmanaged:\n  enabled: true\n  override:\n")
	fmt.Fprintf(&template, "    - file_option: go_package_prefix\n      value: %s\n", frameworkImport+"gen")
	for _, t := range tasks {
		fmt.Fprintf(&template, "    - file_option: go_package_prefix\n      path: %s\n      value: %s\n", strings.TrimSuffix(t.Prefix, "/"), layout.importPath(t.GoOutput))
	}
	template.WriteString("plugins:\n")
	for _, p := range protoPlugins {
		fmt.Fprintf(&template, "  - remote: %s\n    out: out/%s\n    opt: %s\n", p.remote, p.out, p.opt)
	}
	for name, data := range map[string]string{"buf.yaml": workspace.String(), "buf.gen.yaml": template.String()} {
		if err := os.WriteFile(filepath.Join(temp, name), []byte(data), 0600); err != nil {
			return err
		}
	}
	return nil
}

func runBuf(ctx context.Context, temp string) error {
	run := exec.CommandContext(ctx, "buf", "generate", "--template", "buf.gen.yaml")
	run.Dir = temp
	var stderr bytes.Buffer
	run.Stdout, run.Stderr = os.Stderr, &stderr
	if err := run.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("buf is required to generate protocol code: https://buf.build/docs/installation")
		}
		return fmt.Errorf("buf generate: %w\n%s", err, stderr.String())
	}
	return nil
}

// collectProtocol reads the target module's buf output and rewrites its
// TypeScript imports for the application layout.
func collectProtocol(temp string, tasks []generation, task generation) (map[string][]byte, map[string][]byte, error) {
	goFiles, err := relativeFiles(filepath.Join(temp, "out/go"))
	if err != nil {
		return nil, nil, err
	}
	tsFiles, err := relativeFiles(filepath.Join(temp, "out/ts"))
	if err != nil {
		return nil, nil, err
	}
	systemFiles, moduleFiles := protoOwners(tsFiles, tasks)
	goOutput, err := readPrefixed(filepath.Join(temp, "out/go"), goFiles, task.Prefix)
	if err != nil {
		return nil, nil, err
	}
	tsData, err := readPrefixed(filepath.Join(temp, "out/ts"), tsFiles, task.Prefix)
	if err != nil {
		return nil, nil, err
	}
	tsSources := make(map[string]string, len(tsData))
	for file, data := range tsData {
		tsSources[file] = string(data)
	}
	tsOutput, err := rewriteProtoImports(tsSources, systemFiles, moduleFiles, task.TSOutput)
	if err != nil {
		return nil, nil, err
	}
	return goOutput, tsOutput, nil
}

// protoOwners classifies TypeScript output as framework system files or
// files owned by a module, mapped to their application output path.
func protoOwners(tsFiles []string, tasks []generation) (map[string]bool, map[string]string) {
	systemFiles := map[string]bool{}
	moduleFiles := map[string]string{}
	for _, file := range tsFiles {
		if strings.HasPrefix(file, systemProtoPath) {
			systemFiles[file] = true
		}
		for _, t := range tasks {
			if strings.HasPrefix(file, t.Prefix) {
				moduleFiles[file] = path.Join(t.TSOutput, file)
			}
		}
	}
	return systemFiles, moduleFiles
}

func readPrefixed(dir string, files []string, prefix string) (map[string][]byte, error) {
	output := map[string][]byte{}
	for _, file := range files {
		if !strings.HasPrefix(file, prefix) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			return nil, err
		}
		output[file] = data
	}
	return output, nil
}

// writeProtocol replaces the module's managed Go and TypeScript output,
// validating every destination before updating any of them.
func writeProtocol(root string, task generation, goOutput, tsOutput map[string][]byte) error {
	if len(goOutput) == 0 || len(tsOutput) == 0 {
		for _, dir := range []string{task.GoOutput, task.TSOutput} {
			existing, err := relativeFiles(filepath.Join(root, filepath.FromSlash(dir), filepath.FromSlash(task.Prefix)))
			if err != nil {
				return err
			}
			if len(existing) > 0 {
				return fmt.Errorf("module %s has generated protocol files but no protocol sources", task.Name)
			}
		}
		return nil
	}
	destinations := []struct {
		dir   string
		files map[string][]byte
	}{{task.GoOutput, goOutput}, {task.TSOutput, tsOutput}}
	for _, dest := range destinations {
		if err := checkManaged(filepath.Join(root, filepath.FromSlash(dest.dir)), task.Prefix, dest.files); err != nil {
			return err
		}
	}
	for _, dest := range destinations {
		if err := replaceManaged(filepath.Join(root, filepath.FromSlash(dest.dir)), task.Prefix, dest.files); err != nil {
			return err
		}
	}
	return nil
}

// frameworkProtoDir locates the framework's public Proto sources: the
// repository itself, or the framework module required by the application.
func frameworkProtoDir(ctx context.Context, root string, layout appLayout) (string, error) {
	if layout.Framework {
		return filepath.Join(root, "proto"), nil
	}
	list := exec.CommandContext(ctx, "go", "list", "-m", "-f", "{{.Dir}}", frameworkModule)
	list.Dir = root
	var stderr bytes.Buffer
	list.Stderr = &stderr
	out, err := list.Output()
	dir := strings.TrimSpace(string(out))
	if err != nil || dir == "" {
		return "", fmt.Errorf("locate %s sources (run go mod download): %v %s", frameworkModule, err, strings.TrimSpace(stderr.String()))
	}
	return filepath.Join(dir, "proto"), nil
}

func sameDir(a, b string) (bool, error) {
	ai, err := os.Stat(a)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	return os.SameFile(ai, bi), nil
}

// copyProtos copies the .proto files below src, keeping their relative paths.
func copyProtos(src, dst string) error {
	return filepath.WalkDir(src, func(file string, entry fs.DirEntry, err error) error {
		if os.IsNotExist(err) && file == src {
			return nil
		}
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(file, ".proto") {
			return nil
		}
		rel, err := filepath.Rel(src, file)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0600)
	})
}

// relativeFiles lists regular files below dir with slash-separated paths.
func relativeFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(file string, entry fs.DirEntry, err error) error {
		if os.IsNotExist(err) && file == dir {
			return nil
		}
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, file)
		files = append(files, filepath.ToSlash(rel))
		return err
	})
	sort.Strings(files)
	return files, err
}

var generatedMarker = regexp.MustCompile(`^// (Code generated by |@generated by )`)

func checkManaged(dest, prefix string, output map[string][]byte) error {
	existing, err := relativeFiles(filepath.Join(dest, filepath.FromSlash(prefix)))
	if err != nil {
		return err
	}
	files := map[string]bool{}
	for _, f := range existing {
		files[prefix+f] = true
	}
	for f := range output {
		files[f] = true
	}
	for f := range files {
		data, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(f)))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !generatedMarker.Match(data) {
			return fmt.Errorf("unmanaged destination: %s", filepath.Join(dest, filepath.FromSlash(f)))
		}
	}
	return nil
}

func replaceManaged(dest, prefix string, output map[string][]byte) error {
	for f, data := range output {
		full := filepath.Join(dest, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(full, data, 0644); err != nil {
			return err
		}
	}
	existing, err := relativeFiles(filepath.Join(dest, filepath.FromSlash(prefix)))
	if err != nil {
		return err
	}
	for _, f := range existing {
		if _, keep := output[prefix+f]; !keep {
			if err := os.Remove(filepath.Join(dest, filepath.FromSlash(prefix+f))); err != nil {
				return err
			}
		}
	}
	return nil
}

var protoImport = regexp.MustCompile(`(?m)^((?:import|export) [^\n]* from ")(\.[^"]*)(")`)

// rewriteProtoImports points relative imports of protoc-gen-es output at the
// published system package, the same module, or another module's output.
// It follows frontend/tools/rewrite-proto-imports.mjs.
func rewriteProtoImports(files map[string]string, systemFiles map[string]bool, moduleFiles map[string]string, outputDir string) (map[string][]byte, error) {
	output := map[string][]byte{}
	for file, source := range files {
		var rewriteErr error
		text := protoImport.ReplaceAllStringFunc(source, func(match string) string {
			parts := protoImport.FindStringSubmatch(match)
			spec := parts[2]
			bare := strings.TrimSuffix(strings.TrimSuffix(spec, ".js"), ".ts")
			target := path.Clean(path.Join(path.Dir(file), bare)) + ".ts"
			var replacement string
			_, local := files[target]
			switch {
			case strings.HasPrefix(target, systemProtoPath) && !strings.HasPrefix(file, systemProtoPath) && systemFiles[target]:
				replacement = "@abysstract/admin-proto/" + strings.TrimSuffix(target, ".ts")
			case local:
				replacement = bare + ".js"
			case moduleFiles[target] != "":
				rel, err := relativeImport(path.Dir(path.Join(outputDir, file)), moduleFiles[target])
				if err != nil {
					rewriteErr = err
					return match
				}
				replacement = strings.TrimSuffix(rel, ".ts") + ".js"
			default:
				rewriteErr = fmt.Errorf("%s: unknown relative Proto dependency %s (%s)", file, spec, target)
				return match
			}
			return parts[1] + replacement + parts[3]
		})
		if rewriteErr != nil {
			return nil, rewriteErr
		}
		output[file] = []byte(text)
	}
	return output, nil
}
