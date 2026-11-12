package system

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/reflect/protoreflect"
	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/service"
	"github.com/abysstract/abysstract-admin/backend/module"
)

// ErrMissingSystemTables is returned when Create runs against a database that
// has not been migrated.
var ErrMissingSystemTables = errors.New("system: database is missing sys_user; run migrate first")

// ErrMissingStorage is returned when Create runs without module.Runtime.Storage.
var ErrMissingStorage = errors.New("system: module.Runtime.Storage is nil")

func createSystem(ctx context.Context, rt module.Runtime) (module.Instance, error) {
	if err := ctx.Err(); err != nil {
		return module.Instance{}, err
	}
	if err := requireSystemDB(rt.DB); err != nil {
		return module.Instance{}, err
	}
	handlers, err := newSystemHandlers(rt)
	if err != nil {
		return module.Instance{}, err
	}
	services, err := bindSystemServices(handlers)
	if err != nil {
		return module.Instance{}, err
	}
	return module.Instance{
		Services: services,
		Close:    func(context.Context) error { return nil },
	}, nil
}

func requireSystemDB(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable("sys_user") {
		return ErrMissingSystemTables
	}
	return nil
}

type systemHandlers struct {
	auth      adminv1connect.AuthServiceHandler
	user      adminv1connect.UserServiceHandler
	role      adminv1connect.RoleServiceHandler
	menu      adminv1connect.MenuServiceHandler
	dept      adminv1connect.DeptServiceHandler
	dict      adminv1connect.DictServiceHandler
	config    adminv1connect.ConfigServiceHandler
	notice    adminv1connect.NoticeServiceHandler
	log       adminv1connect.LogServiceHandler
	upload    adminv1connect.UploadServiceHandler
	export    adminv1connect.CommonExportServiceHandler
	profile   adminv1connect.ProfileServiceHandler
	dashboard adminv1connect.DashboardServiceHandler
}

func newSystemHandlers(rt module.Runtime) (*systemHandlers, error) {
	cfg := rt.Config
	store := repo.New(rt.DB)
	jwtMgr := jwt.NewManager(cfg.JWTSecret, cfg.JWTExpiresIn)
	loginSec := repo.NewLoginSecurity(rt.DB, cfg.LoginMaxFailures, cfg.LoginLockoutSeconds)
	refresh := repo.NewRefreshTokenRepo(rt.DB)
	if rt.Storage == nil {
		return nil, ErrMissingStorage
	}
	dictSvc, err := service.NewGeneratedDictService(store)
	if err != nil {
		return nil, err
	}
	configSvc, err := service.NewGeneratedConfigService(store)
	if err != nil {
		return nil, err
	}
	noticeSvc, err := service.NewGeneratedNoticeService(store)
	if err != nil {
		return nil, err
	}
	deptSvc, err := service.NewGeneratedDeptService(store)
	if err != nil {
		return nil, err
	}
	menuSvc, err := service.NewGeneratedMenuService(store)
	if err != nil {
		return nil, err
	}
	roleSvc, err := service.NewGeneratedRoleService(store)
	if err != nil {
		return nil, err
	}
	userSvc, err := service.NewGeneratedUserService(store, refresh)
	if err != nil {
		return nil, err
	}
	authSvc := service.NewAuthService(
		store, loginSec, refresh, jwtMgr,
		cfg.JWTExpiresIn,
		cfg.RefreshTokenEnabled, cfg.JWTRefreshExpiresIn,
		cfg.TrustProxy,
	)
	if !service.AccessLogReadable(rt.AccessLog) {
		// Without a Reader the request log page cannot show anything.
		authSvc.HideMenus(service.AccessLogListPerm)
	}
	return &systemHandlers{
		auth:   authSvc,
		user:   userSvc,
		role:   roleSvc,
		menu:   menuSvc,
		dept:   deptSvc,
		dict:   dictSvc,
		config: configSvc,
		notice: noticeSvc,
		log:    service.NewLogService(store, rt.AccessLog),
		upload: service.NewUploadService(store, rt.Storage, service.UploadConfig{
			FileMaxBytes:   cfg.UploadMaxBytes,
			AvatarMaxBytes: cfg.AvatarMaxBytes,
			UploadTTL:      cfg.StorageUploadTTLOrDefault(),
		}),
		export:    service.NewCommonExportService(store, service.NewPermChecker(store.PermsOf)),
		profile:   service.NewProfileService(store),
		dashboard: service.NewDashboardService(store),
	}, nil
}

func bindSystemServices(handlers *systemHandlers) ([]module.ConnectService, error) {
	declared, err := declaredServices()
	if err != nil {
		return nil, err
	}
	byName := map[string]module.ConnectService{
		"abysstract.admin.v1.AuthService":         connectService(declared, "AuthService", adminv1connect.RegisterAuthServiceHandler, handlers.auth),
		"abysstract.admin.v1.UserService":         connectService(declared, "UserService", adminv1connect.RegisterUserServiceHandler, handlers.user),
		"abysstract.admin.v1.RoleService":         connectService(declared, "RoleService", adminv1connect.RegisterRoleServiceHandler, handlers.role),
		"abysstract.admin.v1.MenuService":         connectService(declared, "MenuService", adminv1connect.RegisterMenuServiceHandler, handlers.menu),
		"abysstract.admin.v1.DeptService":         connectService(declared, "DeptService", adminv1connect.RegisterDeptServiceHandler, handlers.dept),
		"abysstract.admin.v1.DictService":         connectService(declared, "DictService", adminv1connect.RegisterDictServiceHandler, handlers.dict),
		"abysstract.admin.v1.ConfigService":       connectService(declared, "ConfigService", adminv1connect.RegisterConfigServiceHandler, handlers.config),
		"abysstract.admin.v1.NoticeService":       connectService(declared, "NoticeService", adminv1connect.RegisterNoticeServiceHandler, handlers.notice),
		"abysstract.admin.v1.LogService":          connectService(declared, "LogService", adminv1connect.RegisterLogServiceHandler, handlers.log),
		"abysstract.admin.v1.UploadService":       connectService(declared, "UploadService", adminv1connect.RegisterUploadServiceHandler, handlers.upload),
		"abysstract.admin.v1.CommonExportService": connectService(declared, "CommonExportService", adminv1connect.RegisterCommonExportServiceHandler, handlers.export),
		"abysstract.admin.v1.ProfileService":      connectService(declared, "ProfileService", adminv1connect.RegisterProfileServiceHandler, handlers.profile),
		"abysstract.admin.v1.DashboardService":    connectService(declared, "DashboardService", adminv1connect.RegisterDashboardServiceHandler, handlers.dashboard),
	}
	out := make([]module.ConnectService, len(declared))
	for i, spec := range declared {
		name := string(spec.Descriptor.FullName())
		bound, ok := byName[name]
		if !ok || bound.Register == nil {
			return nil, fmt.Errorf("system: missing handler for %s", name)
		}
		out[i] = bound
	}
	return out, nil
}

func connectService[T any](
	declared []module.ConnectService,
	shortName string,
	register func(*connect.Server, T),
	handler T,
) module.ConnectService {
	desc := descriptorByShortName(declared, shortName)
	return module.ConnectService{
		Descriptor: desc,
		Register: func(server *connect.Server) {
			register(server, handler)
		},
	}
}

func descriptorByShortName(declared []module.ConnectService, shortName string) protoreflect.ServiceDescriptor {
	for _, spec := range declared {
		if spec.Descriptor != nil && string(spec.Descriptor.Name()) == shortName {
			return spec.Descriptor
		}
	}
	return nil
}
