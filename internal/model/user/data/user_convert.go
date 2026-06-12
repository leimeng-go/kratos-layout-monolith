package data

import (
	"time"

	"github.com/go-kratos/kratos-layout-monolith/internal/model/user/biz"
	"github.com/go-kratos/kratos-layout-monolith/internal/pkg/model"
)

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
		BaseModel: model.BaseModel{
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
