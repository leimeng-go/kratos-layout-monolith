package data

import (
	"github.com/go-kratos/kratos-layout-monolith/internal/moduser/biz"
	"github.com/go-kratos/kratos-layout-monolith/internal/pkg/cache"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"gorm.io/gorm"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewUserRepo)

// NewUserRepo creates a new cached user repository.
func NewUserRepo(db *gorm.DB, rds *cache.Redis, logger log.Logger) biz.UserRepo {
	return newUserRepo(db, rds, logger)
}
