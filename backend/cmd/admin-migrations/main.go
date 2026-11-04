// Command admin-migrations imports Core migration sources into an application.
package main

import (
	"fmt"
	"os"

	"github.com/abysstract/abysstract-admin/backend/adminconfig"
	"github.com/abysstract/abysstract-admin/backend/coremigrations"
)

func main() {
	if len(os.Args) != 4 || os.Args[1] != "sync" || os.Args[2] != "--dir" || os.Args[3] == "" {
		fmt.Fprintln(os.Stderr, "usage: admin-migrations sync --dir <migrations-directory>")
		os.Exit(2)
	}
	cfg := adminconfig.Load()
	if err := cfg.ValidateDatabaseDriver(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	files, err := coremigrations.Sync(os.Args[3], cfg.DBDriver)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, file := range files {
		fmt.Println(file)
	}
}
