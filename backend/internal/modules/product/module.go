package product

import (
	"context"
	"fmt"

	productdef "github.com/abysstract/abysstract-admin/backend/internal/modules/product/definition"

	"connectrpc.com/connect/v2"

	productv1 "github.com/abysstract/abysstract-admin/backend/internal/modules/product/gen/proto/company/product/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/modules/product/gen/proto/company/product/v1/productv1connect"
	gen "github.com/abysstract/abysstract-admin/backend/internal/modules/product/gen/resource"
	"github.com/abysstract/abysstract-admin/backend/module"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

const moduleName = "product"

// NewModule returns the product business module declaration.
func NewModule() (module.Definition, error) {
	return NewModuleWithOptions(ModuleOptions{})
}

// ModuleOptions customizes hooks for tests. Production uses zero values.
type ModuleOptions struct {
	EventWriter EventWriter
	CommitStats *CommitStats
}

// NewModuleWithOptions returns the product module with optional test hooks.
func NewModuleWithOptions(opts ModuleOptions) (module.Definition, error) {
	desc := productv1.File_company_product_v1_product_proto.Services().ByName("ProductService")
	if desc == nil {
		return module.Definition{}, fmt.Errorf("product: missing ProductService descriptor")
	}
	if _, err := productdef.ProductResource.Build(); err != nil {
		return module.Definition{}, fmt.Errorf("product: schema: %w", err)
	}
	return module.Definition{
		Name:        moduleName,
		Requires:    []string{"system"},
		Services:    []module.ConnectService{{Descriptor: desc}},
		Permissions: gen.RegisterProductPermissions,
		Create:      createProduct(opts),
	}, nil
}

func createProduct(opts ModuleOptions) func(context.Context, module.Runtime) (module.Instance, error) {
	return func(ctx context.Context, rt module.Runtime) (module.Instance, error) {
		if err := ctx.Err(); err != nil {
			return module.Instance{}, err
		}
		if rt.DB == nil {
			return module.Instance{}, fmt.Errorf("product: database is required")
		}
		if rt.DataScope == nil {
			return module.Instance{}, fmt.Errorf("product: DataScope provider is required")
		}
		plan, err := productdef.ProductResource.Build()
		if err != nil {
			return module.Instance{}, err
		}
		store, err := resource.NewStore(rt.DB)
		if err != nil {
			return module.Instance{}, err
		}
		controller, err := gen.NewProductController(gen.ProductControllerDeps{
			Store:                store,
			Hooks:                newProductHooks(hookOptions{writeEvent: opts.EventWriter, observe: opts.CommitStats}),
			DataScopeProvider:    rt.DataScope,
			CreateScopeValidator: newCreateScopeValidator(rt.DataScope, plan.DeptDBName, plan.CreatorDBName),
		})
		if err != nil {
			return module.Instance{}, err
		}
		adapter, err := gen.NewProductServiceAdapter(controller)
		if err != nil {
			return module.Instance{}, err
		}
		desc := productv1.File_company_product_v1_product_proto.Services().ByName("ProductService")
		service := module.ConnectService{
			Descriptor: desc,
			Register: func(server *connect.Server) {
				productv1connect.RegisterProductServiceHandler(server, adapter)
			},
		}
		return module.Instance{
			Services: []module.ConnectService{service},
			Close:    func(context.Context) error { return nil },
		}, nil
	}
}
