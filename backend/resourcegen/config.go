package resourcegen

import (
	"fmt"
	"go/token"
	"path"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/abysstract/abysstract-admin/backend/resource"
)

// Config describes one Generate invocation. Output directories are relative
// to the absolute RootDir. Callers supply the resource list explicitly.
type Config struct {
	FormatVersion uint32
	// GeneratorVersion is optional. Empty values resolve from
	// ADMIN_GENERATOR_VERSION, then Go build info, then "dev".
	GeneratorVersion string
	// SourceIdentity is optional. Empty values resolve from
	// ADMIN_GENERATOR_SOURCE. Official versions without a source identity
	// are recorded as development builds.
	SourceIdentity string
	RootDir        string
	GoOutputDir    string
	GoPackage      string
	TSOutputDir    string
	GoImports      GoImports
	TSImports      TSImports
	// GenQueryImport is the required query package used by generated List and Update implementations.
	// Query objects use the model type names produced by ApplyBasic/GenerateModelAs.
	GenQueryImport string
}

// GoImports names the public resource and permission packages. Model, request,
// response, and handler import paths are derived from the validated types.
type GoImports struct {
	Resource   string
	Permission string
}

// TSImports names the public frontend type entries and maps each referenced
// Proto source file to a TypeScript module.
type TSImports struct {
	Schema       string
	Operations   string
	Relation     string
	ProtoModules map[string]string
}

func validateConfig(cfg Config) error {
	if strings.TrimSpace(cfg.GenQueryImport) == "" {
		return fmt.Errorf("resourcegen: GenQueryImport is required")
	}
	if cfg.FormatVersion == 0 {
		return fmt.Errorf("resourcegen: FormatVersion is required")
	}
	if cfg.FormatVersion != resource.GeneratorFormatV1 {
		return fmt.Errorf("resourcegen: unsupported FormatVersion %d", cfg.FormatVersion)
	}
	if !filepath.IsAbs(cfg.RootDir) {
		return fmt.Errorf("resourcegen: RootDir must be an absolute path")
	}
	if err := validateRelativeOutput(cfg.GoOutputDir, "GoOutputDir"); err != nil {
		return err
	}
	if err := validateRelativeOutput(cfg.TSOutputDir, "TSOutputDir"); err != nil {
		return err
	}
	if !token.IsIdentifier(cfg.GoPackage) || !isExportedOrIdent(cfg.GoPackage) {
		return fmt.Errorf("resourcegen: GoPackage %q is not a valid package name", cfg.GoPackage)
	}
	if err := validatePublicImport(cfg.GoImports.Resource, "GoImports.Resource"); err != nil {
		return err
	}
	if err := validatePublicImport(cfg.GoImports.Permission, "GoImports.Permission"); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.TSImports.Schema) == "" {
		return fmt.Errorf("resourcegen: TSImports.Schema is required")
	}
	if strings.TrimSpace(cfg.TSImports.Operations) == "" {
		return fmt.Errorf("resourcegen: TSImports.Operations is required")
	}
	if strings.TrimSpace(cfg.TSImports.Relation) == "" {
		return fmt.Errorf("resourcegen: TSImports.Relation is required")
	}
	return nil
}

func isExportedOrIdent(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsLetter(r) && name != "main" && name != "_"
}

func validateRelativeOutput(value, field string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("resourcegen: %s is required", field)
	}
	if filepath.IsAbs(value) {
		return fmt.Errorf("resourcegen: %s must be a relative path", field)
	}
	cleaned := path.Clean(slashPath(value))
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("resourcegen: %s escapes RootDir", field)
	}
	for _, segment := range strings.Split(cleaned, "/") {
		if segment == ".." {
			return fmt.Errorf("resourcegen: %s escapes RootDir", field)
		}
	}
	return nil
}

func validatePublicImport(value, field string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("resourcegen: %s is required", field)
	}
	if hasInternalSegment(value) {
		return fmt.Errorf("resourcegen: %s must be a public import path", field)
	}
	return nil
}

func hasInternalSegment(importPath string) bool {
	for _, segment := range strings.Split(importPath, "/") {
		if segment == "internal" {
			return true
		}
	}
	return false
}

func slashPath(value string) string {
	return strings.ReplaceAll(value, "\\", "/")
}
