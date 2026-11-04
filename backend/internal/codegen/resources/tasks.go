// Package resources declares the application's generation tasks.
package resources

import (
	"github.com/abysstract/abysstract-admin/backend/resource/schema"
	"github.com/abysstract/abysstract-admin/backend/resourcegen"
)

type Task struct {
	Name      string
	Config    resourcegen.Config
	Resources []*schema.Resource
}
