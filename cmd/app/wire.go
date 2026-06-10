//go:build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

//go:generate go run -mod=mod github.com/google/wire/cmd/wire

import (
	"github.com/go-kratos/kratos-layout-monolith/internal/conf"
	"github.com/go-kratos/kratos-layout-monolith/internal/model/user"
	"github.com/go-kratos/kratos-layout-monolith/internal/pkg/cache"
	"github.com/go-kratos/kratos-layout-monolith/internal/pkg/db"
	"github.com/go-kratos/kratos-layout-monolith/internal/pkg/lock"
	"github.com/go-kratos/kratos-layout-monolith/internal/server"

	kratoslog "github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

func initApp(*conf.Bootstrap, kratoslog.Logger) (*appComponents, func(), error) {
	panic(wire.Build(
		conf.ProviderSet,
		db.ProviderSet,
		cache.ProviderSet,
		cache.CachedDBProviderSet,
		lock.ProviderSet,
		user.ProviderSet,
		server.ProviderSet,
		newAppComponents,
	))
}
