package data

import (
	"github.com/go-kratos/kratos-layout-monolith/internal/model/user/biz"
	"github.com/go-kratos/kratos-layout-monolith/internal/pkg/cache"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewUserRepo)

func NewUserRepo(cdb *cache.CachedDB, logger log.Logger) biz.UserRepo {
	return newUserRepo(cdb, logger)
}
