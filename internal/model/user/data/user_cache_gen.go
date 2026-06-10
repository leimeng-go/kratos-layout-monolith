package data

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos-layout-monolith/internal/model/user/biz"
	"github.com/go-kratos/kratos-layout-monolith/internal/pkg/cache"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	userCacheIdPrefix       = "cache:users:id:"
	userCacheUsernamePrefix = "cache:users:username:"
	userCacheEmailPrefix    = "cache:users:email:"
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

func userCacheIdKey(id int64) string {
	return fmt.Sprintf("%s%d", userCacheIdPrefix, id)
}

func userCacheUsernameKey(username string) string {
	return fmt.Sprintf("%s%s", userCacheUsernamePrefix, username)
}

func userCacheEmailKey(email string) string {
	return fmt.Sprintf("%s%s", userCacheEmailPrefix, email)
}

func (r *userRepo) modelCacheKeys(data *User) []string {
	keys := []string{
		userCacheIdKey(data.Id),
	}
	if data.Username != "" {
		keys = append(keys, userCacheUsernameKey(data.Username))
	}
	if data.Email != "" {
		keys = append(keys, userCacheEmailKey(data.Email))
	}
	return keys
}

func toBizUser(u *User) *biz.User {
	return &biz.User{
		Id:        u.Id,
		Username:  u.Username,
		Email:     u.Email,
		Phone:     u.Phone,
		Nickname:  u.Nickname,
		Avatar:    u.Avatar,
		Status:    u.Status,
		Version:   u.Version,
		CreatedAt: u.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: u.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func fromBizUser(u *biz.User) *User {
	return &User{
		BaseModel: BaseModel{
			Id:        u.Id,
			Version:   u.Version,
			CreatedAt: mustParseTime(u.CreatedAt),
			UpdatedAt: mustParseTime(u.UpdatedAt),
		},
		Username: u.Username,
		Email:    u.Email,
		Phone:    u.Phone,
		Nickname: u.Nickname,
		Avatar:   u.Avatar,
		Status:   u.Status,
	}
}

func mustParseTime(s string) time.Time {
	t, _ := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local)
	return t
}

func (r *userRepo) CreateUser(ctx context.Context, u *biz.User) (*biz.User, error) {
	user := fromBizUser(u)
	user.DelState = DelStateNo
	err := r.cdb.Exec(ctx, func() error {
		return r.cdb.DBCtx(ctx).Create(user).Error
	})
	if err != nil {
		return nil, err
	}
	return toBizUser(user), nil
}

func (r *userRepo) UpdateUser(ctx context.Context, u *biz.User) (*biz.User, error) {
	old := &User{}
	if err := r.cdb.DBCtx(ctx).Where("del_state = ?", DelStateNo).First(old, u.Id).Error; err != nil {
		return nil, err
	}
	oldKeys := r.modelCacheKeys(old)

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
	}, oldKeys...)
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
		return r.cdb.DBCtx(ctx).Where("del_state = ?", DelStateNo).First(&user, id).Error
	})
	if err != nil {
		return nil, err
	}
	return toBizUser(&user), nil
}

func (r *userRepo) GetUserByUsername(ctx context.Context, username string) (*biz.User, error) {
	var user User
	err := r.cdb.QueryRowIndex(ctx, &user,
		userCacheUsernameKey(username),
		func() (string, error) {
			var id int64
			err := r.cdb.DBCtx(ctx).Model(&User{}).Where("username = ? AND del_state = ?", username, DelStateNo).Select("id").Scan(&id).Error
			if err != nil {
				return "", err
			}
			if id == 0 {
				return "", cache.ErrNotFound
			}
			return userCacheIdKey(id), nil
		},
		func() error {
			return r.cdb.DBCtx(ctx).Where("del_state = ?", DelStateNo).First(&user, user.Id).Error
		},
	)
	if err != nil {
		return nil, err
	}
	return toBizUser(&user), nil
}

func (r *userRepo) GetUserByEmail(ctx context.Context, email string) (*biz.User, error) {
	var user User
	err := r.cdb.QueryRowIndex(ctx, &user,
		userCacheEmailKey(email),
		func() (string, error) {
			var id int64
			err := r.cdb.DBCtx(ctx).Model(&User{}).Where("email = ? AND del_state = ?", email, DelStateNo).Select("id").Scan(&id).Error
			if err != nil {
				return "", err
			}
			if id == 0 {
				return "", cache.ErrNotFound
			}
			return userCacheIdKey(id), nil
		},
		func() error {
			return r.cdb.DBCtx(ctx).Where("del_state = ?", DelStateNo).First(&user, user.Id).Error
		},
	)
	if err != nil {
		return nil, err
	}
	return toBizUser(&user), nil
}

func (r *userRepo) ListUsers(ctx context.Context, page, pageSize int32) ([]*biz.User, int32, error) {
	var users []User
	var total int64

	db := r.cdb.DBCtx(ctx).Where("del_state = ?", DelStateNo)
	if err := db.Model(&User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := db.Offset(int(offset)).Limit(int(pageSize)).Order("id DESC").Find(&users).Error; err != nil {
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

	db := r.cdb.DBCtx(ctx).Where("del_state = ?", DelStateNo)
	if lastId > 0 {
		db = db.Where("id < ?", lastId)
	}
	if err := db.Order("id DESC").Limit(int(pageSize)).Find(&users).Error; err != nil {
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
	if err := r.cdb.DBCtx(ctx).Where("del_state = ?", DelStateNo).First(old, id).Error; err != nil {
		return err
	}

	now := time.Now()
	return r.cdb.Exec(ctx, func() error {
		result := r.cdb.DBCtx(ctx).Model(&User{}).Where("id = ? AND version = ?", id, old.Version).Updates(map[string]any{
			"del_state":  DelStateYes,
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
