package data

import (
	"context"
	"errors"
	"time"

	"github.com/go-kratos/kratos-layout-monolith/internal/model/user/biz"
	"github.com/go-kratos/kratos-layout-monolith/internal/pkg/cache"
	"github.com/go-kratos/kratos-layout-monolith/internal/pkg/model"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

type userRepo struct {
	cdb *cache.CachedDB
	log *log.Helper
}

func newUserRepo(cdb *cache.CachedDB, logger log.Logger) *userRepo {
	return &userRepo{
		cdb: cdb,
		log: log.NewHelper(logger),
	}
}

func normalizeNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return cache.ErrNotFound
	}
	return err
}

func (r *userRepo) CreateUser(ctx context.Context, u *biz.User) (*biz.User, error) {
	user := fromBizUser(u)
	user.DelState = model.DelStateNo
	keys := r.modelUniqueCacheKeys(user)
	err := r.cdb.Exec(ctx, func() error {
		return r.cdb.DBCtx(ctx).Create(user).Error
	}, keys...)
	if err != nil {
		return nil, err
	}
	if user.Id > 0 {
		if err := r.cdb.DelCache(ctx, userCacheIdKey(user.Id)); err != nil {
			return nil, err
		}
	}
	return toBizUser(user), nil
}

func (r *userRepo) UpdateUser(ctx context.Context, u *biz.User) (*biz.User, error) {
	old := &User{}
	if err := r.cdb.DBCtx(ctx).Where("del_state = ?", model.DelStateNo).First(old, u.Id).Error; err != nil {
		return nil, err
	}
	newData := fromBizUser(u)
	newData.Id = old.Id
	keys := append(r.modelCacheKeys(old), r.modelCacheKeys(newData)...)

	err := r.cdb.Exec(ctx, func() error {
		result := r.cdb.DBCtx(ctx).Model(&User{}).Where("id = ? AND version = ?", u.Id, old.Version).Updates(map[string]any{
			"username": u.Username,
			"email":    u.Email,
			"phone":    u.Phone,
			"nickname": u.Nickname,
			"avatar":   u.Avatar,
			"status":   u.Status,
			"version":  old.Version + 1,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return biz.ErrNoRowsUpdate
		}
		return nil
	}, keys...)
	if err != nil {
		return nil, err
	}

	return &biz.User{
		Id:       u.Id,
		Username: u.Username,
		Email:    u.Email,
		Phone:    u.Phone,
		Nickname: u.Nickname,
		Avatar:   u.Avatar,
		Status:   u.Status,
	}, nil
}

func (r *userRepo) GetUserByID(ctx context.Context, id int64) (*biz.User, error) {
	var user User
	err := r.cdb.QueryRow(ctx, &user, userCacheIdKey(id), func() error {
		return normalizeNotFound(r.cdb.DBCtx(ctx).Where("del_state = ?", model.DelStateNo).First(&user, id).Error)
	})
	if err != nil {
		return nil, err
	}
	return toBizUser(&user), nil
}

func (r *userRepo) getUserByUniqueIndex(ctx context.Context, index userUniqueIndex, value string) (*biz.User, error) {
	var user User
	query := func() error {
		return normalizeNotFound(r.cdb.DBCtx(ctx).Where(index.column+" = ? AND del_state = ?", value, model.DelStateNo).First(&user).Error)
	}
	err := r.cdb.QueryRowIndex(ctx, &user,
		index.key(value),
		func() (string, error) {
			err := query()
			if err != nil {
				return "", err
			}
			if user.Id == 0 {
				return "", cache.ErrNotFound
			}
			return userCacheIdKey(user.Id), nil
		},
		query,
	)
	if err != nil {
		return nil, err
	}
	return toBizUser(&user), nil
}

func (r *userRepo) GetUserByUsername(ctx context.Context, username string) (*biz.User, error) {
	return r.getUserByUniqueIndex(ctx, userUniqueIndexes[0], username)
}

func (r *userRepo) GetUserByEmail(ctx context.Context, email string) (*biz.User, error) {
	return r.getUserByUniqueIndex(ctx, userUniqueIndexes[1], email)
}

func (r *userRepo) ListUsers(ctx context.Context, page, pageSize int32) ([]*biz.User, int32, error) {
	var users []User
	build := func(db *gorm.DB) *gorm.DB {
		return db.Where("del_state = ?", model.DelStateNo)
	}
	total, err := r.cdb.FindPageListByPageWithTotal(ctx, &users, &User{}, build, page, pageSize, "id DESC")
	if err != nil {
		return nil, 0, err
	}

	result := make([]*biz.User, 0, len(users))
	for _, u := range users {
		result = append(result, toBizUser(&u))
	}
	return result, int32(total), nil
}

func (r *userRepo) ListUsersByStatus(ctx context.Context, status, page, pageSize int32) ([]*biz.User, int32, error) {
	var users []User
	build := func(db *gorm.DB) *gorm.DB {
		return db.Where("status = ? AND del_state = ?", status, model.DelStateNo)
	}
	total, err := r.cdb.FindPageListByPageWithTotal(ctx, &users, &User{}, build, page, pageSize, "id DESC")
	if err != nil {
		return nil, 0, err
	}

	result := make([]*biz.User, 0, len(users))
	for _, u := range users {
		result = append(result, toBizUser(&u))
	}
	return result, int32(total), nil
}

func (r *userRepo) ListUsersByIdDesc(ctx context.Context, lastId, pageSize int32) ([]*biz.User, error) {
	var users []User
	build := func(db *gorm.DB) *gorm.DB {
		return db.Where("del_state = ?", model.DelStateNo)
	}
	if err := r.cdb.FindPageListByIdDesc(ctx, &users, build, lastId, pageSize); err != nil {
		return nil, err
	}

	result := make([]*biz.User, 0, len(users))
	for _, u := range users {
		result = append(result, toBizUser(&u))
	}
	return result, nil
}

func (r *userRepo) Trans(ctx context.Context, fn func(ctx context.Context) error) error {
	return r.cdb.Trans(ctx, fn)
}

func (r *userRepo) DeleteUser(ctx context.Context, id int64) error {
	old := &User{}
	if err := r.cdb.DBCtx(ctx).Where("del_state = ?", model.DelStateNo).First(old, id).Error; err != nil {
		return err
	}

	now := time.Now()
	return r.cdb.Exec(ctx, func() error {
		result := r.cdb.DBCtx(ctx).Model(&User{}).Where("id = ? AND version = ?", id, old.Version).Updates(map[string]any{
			"del_state":  model.DelStateYes,
			"deleted_at": &now,
			"version":    old.Version + 1,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return biz.ErrNoRowsUpdate
		}
		return nil
	}, r.modelCacheKeys(old)...)
}
