package biz

import (
	"context"

	v1 "github.com/go-kratos/kratos-layout-monolith/api/user/v1"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

var (
	// ErrUserNotFound is returned when a user is not found.
	ErrUserNotFound = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")
	// ErrUsernameExists is returned when a username already exists.
	ErrUsernameExists = errors.Conflict(v1.ErrorReason_USERNAME_EXISTS.String(), "username already exists")
	// ErrInvalidCredentials is returned when login credentials are invalid.
	ErrInvalidCredentials = errors.Unauthorized(v1.ErrorReason_INVALID_CREDENTIALS.String(), "invalid credentials")
)

// User is the user domain model.
type User struct {
	ID        int64
	Username  string
	Password  string
	Email     string
	Phone     string
	Nickname  string
	Avatar    string
	Status    int32
	CreatedAt string
	UpdatedAt string
}

// UserRepo is the user repository interface.
type UserRepo interface {
	CreateUser(context.Context, *User) (*User, error)
	UpdateUser(context.Context, *User) (*User, error)
	GetUserByID(context.Context, int64) (*User, error)
	GetUserByUsername(context.Context, string) (*User, error)
	ListUsers(context.Context, int32, int32) ([]*User, int32, error)
	DeleteUser(context.Context, int64) error
}

// UserUsecase is the user usecase.
type UserUsecase struct {
	repo UserRepo
	log  *log.Helper
}

// NewUserUsecase creates a new user usecase.
func NewUserUsecase(repo UserRepo, logger log.Logger) *UserUsecase {
	return &UserUsecase{
		repo: repo,
		log:  log.NewHelper(logger),
	}
}

// Register creates a new user and returns the user data.
func (uc *UserUsecase) Register(ctx context.Context, u *User) (*User, error) {
	// Check if username exists
	existing, err := uc.repo.GetUserByUsername(ctx, u.Username)
	if err == nil && existing != nil {
		return nil, ErrUsernameExists
	}

	uc.log.Infof("registering user: %s", u.Username)
	return uc.repo.CreateUser(ctx, u)
}

// Login validates user credentials.
func (uc *UserUsecase) Login(ctx context.Context, username, password string) (*User, error) {
	user, err := uc.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if user.Password != password {
		return nil, ErrInvalidCredentials
	}
	return user, nil
}

// GetUserByID gets a user by ID.
func (uc *UserUsecase) GetUserByID(ctx context.Context, id int64) (*User, error) {
	user, err := uc.repo.GetUserByID(ctx, id)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// ListUsers lists users with pagination.
func (uc *UserUsecase) ListUsers(ctx context.Context, page, pageSize int32) ([]*User, int32, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	return uc.repo.ListUsers(ctx, page, pageSize)
}

// UpdateUser updates a user.
func (uc *UserUsecase) UpdateUser(ctx context.Context, u *User) (*User, error) {
	return uc.repo.UpdateUser(ctx, u)
}

// DeleteUser deletes a user.
func (uc *UserUsecase) DeleteUser(ctx context.Context, id int64) error {
	return uc.repo.DeleteUser(ctx, id)
}
