// Package modelcfg registers application model overrides. Module configuration
// packages may depend on gorm.io/gen and table names, independently of generated code.
package modelcfg

import "gorm.io/gen"

var Overrides = map[string][]gen.ModelOpt{}
