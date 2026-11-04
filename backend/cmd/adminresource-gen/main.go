// adminresource-gen runs the application Resource task list.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/abysstract/abysstract-admin/backend/internal/codegen/resources"
	"github.com/abysstract/abysstract-admin/backend/resourcegen"
)

func main() {
	selected := flag.String("module", "", "generate one discovered module")
	flag.Parse()
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "adminresource-gen:", err)
		os.Exit(1)
	}
	matched := false
	for _, task := range resources.Tasks(root) {
		if *selected != "" && task.Name != *selected {
			continue
		}
		matched = true
		if err := resourcegen.Generate(task.Config, task.Resources); err != nil {
			fmt.Fprintln(os.Stderr, "adminresource-gen:", err)
			os.Exit(1)
		}
	}
	if !matched {
		fmt.Fprintln(os.Stderr, "adminresource-gen: unknown module", *selected)
		os.Exit(1)
	}
}

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		backend := filepath.Join(dir, "backend", "go.mod")
		frontend := filepath.Join(dir, "frontend", "package.json")
		if _, err := os.Stat(backend); err == nil {
			if _, err := os.Stat(frontend); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("未找到仓库根目录")
		}
		dir = parent
	}
}
