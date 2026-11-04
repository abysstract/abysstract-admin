// Command adminapi-check verifies that public Go package declarations do not
// expose implementation types from an internal package in the same module.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: adminapi-check <package-pattern> [...]")
		os.Exit(2)
	}
	findings, err := checkPackages(os.Args[1:], "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "adminapi-check: %v\n", err)
		os.Exit(2)
	}
	if len(findings) > 0 {
		for _, finding := range findings {
			fmt.Fprintln(os.Stdout, finding)
		}
		os.Exit(1)
	}
	fmt.Printf("adminapi-check: %d package(s) passed\n", len(os.Args)-1)
}
