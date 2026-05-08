package data

import (
	"context"

	"github.com/go-kratos/kratos-layout-monolith/internal/moduser/biz"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

type userRepo struct {
	db  *gorm.DB
	log *log.Helper
}

// NewUserRepo creates a new user repository.
func NewUserRepo(db *gorm.DB, logger log.Logger) biz.UserRepo {
	return &userRepo{
		db:  db,
		log: log.NewHelper(logger),
	}
}

func (r *userRepo) CreateUser(ctx context.Context, u *biz.User) (*biz.User, error) {
	user := &User{
		Username: u.Username,
		Password: u.Password,
		Email:    u.Email,
		Phone:    u.Phone,
		Nickname: u.Nickname,
		Avatar:   u.Avatar,
		Status:   u.Status,
	}
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return nil, err
	}
	return toBizUser(user), nil
}

func (r *userRepo) UpdateUser(ctx context.Context, u *biz.User) (*biz.User, error) {
	user := &User{ID: u.ID}
	updates := map[string]interface{}{
		"nickname": u.Nickname,
		"avatar":   u.Avatar,
		"phone":    u.Phone,
		"status":   u.Status,
	}
	if err := r.db.WithContext(ctx).Model(user).Updates(updates).Error; err != nil {
		return nil, err
	}
	return r.GetUserByID(ctx, u.ID)
}

func (r *userRepo) GetUserByID(ctx context.Context, id int64) (*biz.User, error) {
	var user User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	return toBizUser(&user), nil
}

func (r *userRepo) GetUserByUsername(ctx context.Context, username string) (*biz.User, error) {
	var user User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return toBizUser(&user), nil
}

func (r *userRepo) ListUsers(ctx context.Context, page, pageSize int32) ([]*biz.User, int32, error) {
	var users []User
	var total int64

	if err := r.db.WithContext(ctx).Model(&User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := r.db.WithContext(ctx).
		Offset(int(offset)).
		Limit(int(pageSize)).
		Order("id DESC").
		Find(&users).Error; err != nil {
		return nil, 0, err
	}

	result := make([]*biz.User, 0, len(users))
	for _, u := range users {
		result = append(result, toBizUser(&u))
	}
	return result, int32(total), nil
}

func (r *userRepo) DeleteUser(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&User{}, id).Error
}

func toBizUser(u *User) *biz.User {
	return &biz.User{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		Phone:     u.Phone,
		Nickname:  u.Nickname,
		Avatar:    u.Avatar,
		Status:    u.Status,
		CreatedAt: u.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: u.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}
