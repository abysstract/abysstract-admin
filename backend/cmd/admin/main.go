// Command admin manages application development tasks.
package main

import (
	"fmt"
	"os"

	"github.com/abysstract/abysstract-admin/backend/internal/cli"
)

func main() {
	if err := cli.NewCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
