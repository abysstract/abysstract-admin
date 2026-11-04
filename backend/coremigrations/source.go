// Package coremigrations provides reviewable, product-specific migration sources.
package coremigrations

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed postgres/*.sql postgres/*.go.tmpl mysql/*.sql mysql/*.go.tmpl mariadb/*.sql mariadb/*.go.tmpl sqlite/*.sql sqlite/*.go.tmpl
var sources embed.FS

func SourcesFor(driver string) (fs.FS, error) {
	if driver != "postgres" && driver != "mysql" && driver != "mariadb" && driver != "sqlite" {
		return nil, fmt.Errorf("unsupported migration product %q", driver)
	}
	return fs.Sub(sources, driver)
}
