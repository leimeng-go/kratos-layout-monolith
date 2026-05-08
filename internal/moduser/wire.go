//go:build wireinject
// +build wireinject

package moduser

import (
	"github.com/go-kratos/kratos-layout-monolith/internal/conf"
	"github.com/go-kratos/kratos-layout-monolith/internal/moduser/biz"
	"github.com/go-kratos/kratos-layout-monolith/internal/moduser/data"
	"github.com/go-kratos/kratos-layout-monolith/internal/moduser/service"

	v1 "github.com/go-kratos/kratos-layout-monolith/api/user/v1"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/google/wire"
)

// ProviderSet provides all dependencies for the user module.
var ProviderSet = wire.NewSet(
	biz.ProviderSet,
	data.ProviderSet,
	service.NewUserService,
)

// RegisterHTTP registers the user module's HTTP routes on the given server.
// Call this from main.go's RegisterModuleRoutes function.
func RegisterHTTP(
	s *http.Server,
	uc *biz.UserUsecase,
	logger log.Logger,
	jwt *conf.Jwt,
) {
	srv := service.NewUserService(uc, jwt.Secret, jwt.Expire)
	v1.RegisterUserServiceHTTPServer(s, srv)
	log.NewHelper(logger).Infof("[moduser] routes registered")
}
