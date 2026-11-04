// Package moduleprobe compiles a business-owned module definition against the
// public module package. It does not import system configuration or services.
package moduleprobe

import (
	"context"
	"errors"

	"connectrpc.com/connect/v2"

	"github.com/abysstract/abysstract-admin/backend/module"
)

// Config is business-owned configuration captured by DefinitionFor.
type Config struct {
	Prefix string
}

// DefinitionFor demonstrates that a business module can retain strong
// configuration types while receiving only the shared public Runtime.
func DefinitionFor(config Config) module.Definition {
	return module.Definition{
		Name: "product",
		Create: func(ctx context.Context, runtime module.Runtime) (module.Instance, error) {
			if ctx == nil || runtime.DB == nil || runtime.Logger == nil {
				return module.Instance{}, errors.New("module runtime is incomplete")
			}
			service := module.ConnectService{
				Register: func(server *connect.Server) {
					_ = server
				},
			}
			_ = config.Prefix
			return module.Instance{
				Services: []module.ConnectService{service},
				Close: func(context.Context) error {
					return nil
				},
			}, nil
		},
	}
}
