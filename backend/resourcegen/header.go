package resourcegen

import (
	"fmt"
	"os"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/abysstract/abysstract-admin/backend/resource"
)

const (
	devVersionMarker    = "dev"
	generatorVersionEnv = "ADMIN_GENERATOR_VERSION"
	generatorSourceEnv  = "ADMIN_GENERATOR_SOURCE"
)

func toolVersion() string {
	version, _ := resolveGeneratorIdentity(Config{})
	if version == "" {
		return devVersionMarker
	}
	return version
}

func resolveGeneratorIdentity(cfg Config) (version, source string) {
	version = firstNonEmpty(cfg.GeneratorVersion, os.Getenv(generatorVersionEnv), buildInfoVersion())
	source = firstNonEmpty(cfg.SourceIdentity, os.Getenv(generatorSourceEnv))
	if version == "" {
		version = devVersionMarker
	}
	if source == "" && !isDevelopmentVersion(version) {
		version = devVersionMarker
	}
	return version, source
}

func buildInfoVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return ""
	}
	return info.Main.Version
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func isDevelopmentVersion(version string) bool {
	normalized := strings.ToLower(strings.TrimSpace(version))
	normalized = strings.TrimPrefix(normalized, "v")
	if normalized == "" || normalized == devVersionMarker {
		return true
	}
	if strings.HasPrefix(normalized, "dev.") || strings.HasPrefix(normalized, "dev+") {
		return true
	}
	return strings.Contains(normalized, "-dev")
}

func fileHeader(version string, format uint32, source string) string {
	header := generatedHeader + "\n" +
		"// GeneratorVersion: " + version + "\n" +
		"// FormatVersion: " + strconv.FormatUint(uint64(format), 10) + "\n"
	if trimmed := strings.TrimSpace(source); trimmed != "" {
		header += "// SourceIdentity: " + trimmed + "\n"
	}
	return header
}

func fileHeaderFrom(cfg Config) string {
	version, source := resolveGeneratorIdentity(cfg)
	return fileHeader(version, cfg.FormatVersion, source)
}

func formatGoFile(cfg Config, version string) outFile {
	return outFile{
		name: "format.gen.go",
		content: []byte(fmt.Sprintf("%spackage %s\n\nimport %q\n\nvar _ uint32 = resource.GeneratorFormatV1\n",
			fileHeader(version, cfg.FormatVersion, cfg.SourceIdentity),
			cfg.GoPackage,
			cfg.GoImports.Resource,
		)),
	}
}

func formatTSFile(cfg Config, version string) outFile {
	return outFile{
		name: "generator-format.gen.ts",
		content: []byte(fmt.Sprintf("%s\nimport type { GeneratorFormatV1 } from %q;\n\nexport const generatorFormat: GeneratorFormatV1 = %d;\n",
			fileHeader(version, cfg.FormatVersion, cfg.SourceIdentity),
			cfg.TSImports.Schema,
			resource.GeneratorFormatV1,
		)),
	}
}
