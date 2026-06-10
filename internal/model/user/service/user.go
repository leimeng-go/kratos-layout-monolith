package service

import (
	"context"

	v1 "github.com/go-kratos/kratos-layout-monolith/api/user/v1"
	"github.com/go-kratos/kratos-layout-monolith/internal/model/user/biz"
	"github.com/go-kratos/kratos-layout-monolith/internal/middleware/auth"
)

// UserService handles user-related HTTP requests.
type UserService struct {
	v1.UnimplementedUserServiceServer

	uc    *biz.UserUsecase
	secret string
	expire int64
}

// NewUserService creates a new UserService.
func NewUserService(uc *biz.UserUsecase, secret string, expire int64) *UserService {
	return &UserService{
		uc:    uc,
		secret: secret,
		expire: expire,
	}
}

// Register implements UserServiceServer.
func (s *UserService) Register(ctx context.Context, req *v1.RegisterRequest) (*v1.RegisterReply, error) {
	user, err := s.uc.Register(ctx, &biz.User{
		Username: req.Username,
		Password: req.Password,
		Email:    req.Email,
		Phone:    req.Phone,
	})
	if err != nil {
		return nil, err
	}

	token, err := auth.GenerateToken(s.secret, user.Id, user.Username, s.expire)
	if err != nil {
		return nil, err
	}

	return &v1.RegisterReply{
		Id:    user.Id,
		Token: token,
	}, nil
}

// Login implements UserServiceServer.
func (s *UserService) Login(ctx context.Context, req *v1.LoginRequest) (*v1.LoginReply, error) {
	user, err := s.uc.Login(ctx, req.Username, req.Password)
	if err != nil {
		return nil, err
	}

	token, err := auth.GenerateToken(s.secret, user.Id, user.Username, s.expire)
	if err != nil {
		return nil, err
	}

	return &v1.LoginReply{
		Token: token,
		User: toPBUser(user),
	}, nil
}

// GetUser implements UserServiceServer.
func (s *UserService) GetUser(ctx context.Context, req *v1.GetUserRequest) (*v1.GetUserReply, error) {
	user, err := s.uc.GetUserByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &v1.GetUserReply{User: toPBUser(user)}, nil
}

// ListUsers implements UserServiceServer.
func (s *UserService) ListUsers(ctx context.Context, req *v1.ListUsersRequest) (*v1.ListUsersReply, error) {
	users, total, err := s.uc.ListUsers(ctx, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	items := make([]*v1.User, 0, len(users))
	for _, u := range users {
		items = append(items, toPBUser(u))
	}
	return &v1.ListUsersReply{
		Total: total,
		Items: items,
	}, nil
}

// UpdateUser implements UserServiceServer.
func (s *UserService) UpdateUser(ctx context.Context, req *v1.UpdateUserRequest) (*v1.UpdateUserReply, error) {
	_, err := s.uc.UpdateUser(ctx, &biz.User{
		Id:       req.Id,
		Nickname: req.User.Nickname,
		Avatar:   req.User.Avatar,
		Phone:    req.User.Phone,
		Status:   req.User.Status,
	})
	if err != nil {
		return nil, err
	}
	return &v1.UpdateUserReply{}, nil
}

// DeleteUser implements UserServiceServer.
func (s *UserService) DeleteUser(ctx context.Context, req *v1.DeleteUserRequest) (*v1.DeleteUserReply, error) {
	if err := s.uc.DeleteUser(ctx, req.Id); err != nil {
		return nil, err
	}
	return &v1.DeleteUserReply{}, nil
}

func toPBUser(u *biz.User) *v1.User {
	return &v1.User{
		Id:        u.Id,
		Username:  u.Username,
		Email:     u.Email,
		Phone:     u.Phone,
		Nickname:  u.Nickname,
		Avatar:    u.Avatar,
		Status:    u.Status,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}
