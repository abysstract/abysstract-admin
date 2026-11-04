package app

import "github.com/abysstract/abysstract-admin/backend/module"

// Modules selects application modules from the generated installation list.
func Modules() ([]module.Definition, error) { return selectedModules() }
