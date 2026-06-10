package biz

import (
	"context"
	stderrors "errors"
	"time"

	v1 "github.com/go-kratos/kratos-layout-monolith/api/user/v1"
	"github.com/go-kratos/kratos-layout-monolith/internal/pkg/lock"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

var (
	ErrUserNotFound    = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")
	ErrUsernameExists  = errors.Conflict(v1.ErrorReason_USERNAME_EXISTS.String(), "username already exists")
	ErrInvalidCredentials = errors.Unauthorized(v1.ErrorReason_INVALID_CREDENTIALS.String(), "invalid credentials")
	ErrTooManyRequests = errors.New(429, "TOO_MANY_REQUESTS", "request too frequent, please try again later")
	ErrConcurrentUpdate = errors.Conflict("CONCURRENT_UPDATE", "data has been modified by another request, please retry")
	ErrNoRowsUpdate     = errors.New(500, "NO_ROWS_UPDATE", "affected rows is 0, data may be stale")
)

type User struct {
	Id        int64
	Username  string
	Password  string
	Email     string
	Phone     string
	Nickname  string
	Avatar    string
	Status    int32
	Version   int64
	CreatedAt string
	UpdatedAt string
}

type UserRepo interface {
	CreateUser(context.Context, *User) (*User, error)
	UpdateUser(context.Context, *User) (*User, error)
	GetUserByID(context.Context, int64) (*User, error)
	GetUserByUsername(context.Context, string) (*User, error)
	ListUsers(context.Context, int32, int32) ([]*User, int32, error)
	ListUsersByIdDesc(ctx context.Context, lastId, pageSize int32) ([]*User, error)
	DeleteUser(context.Context, int64) error
	Trans(ctx context.Context, fn func(ctx context.Context) error) error
}

// UserUsecase is the user usecase.
type UserUsecase struct {
	repo   UserRepo
	locker lock.Locker
	log    *log.Helper
}

// NewUserUsecase creates a new user usecase.
func NewUserUsecase(repo UserRepo, locker lock.Locker, logger log.Logger) *UserUsecase {
	return &UserUsecase{
		repo:   repo,
		locker: locker,
		log:    log.NewHelper(logger),
	}
}

// Register creates a new user and returns the user data.
func (uc *UserUsecase) Register(ctx context.Context, u *User) (*User, error) {
	// Acquire distributed lock to prevent concurrent registration with the same username
	lockKey := "lock:register:" + u.Username
	unlock, err := uc.locker.Lock(ctx, lockKey, 5*time.Second)
	if err != nil {
		return nil, ErrTooManyRequests
	}
	defer unlock()

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

func (uc *UserUsecase) ListUsers(ctx context.Context, page, pageSize int32) ([]*User, int32, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	return uc.repo.ListUsers(ctx, page, pageSize)
}

func (uc *UserUsecase) ListUsersByIdDesc(ctx context.Context, lastId, pageSize int32) ([]*User, error) {
	if pageSize <= 0 {
		pageSize = 10
	}
	return uc.repo.ListUsersByIdDesc(ctx, lastId, pageSize)
}

func (uc *UserUsecase) UpdateUser(ctx context.Context, u *User) (*User, error) {
	user, err := uc.repo.UpdateUser(ctx, u)
	if err != nil && stderrors.Is(err, ErrNoRowsUpdate) {
		return nil, ErrConcurrentUpdate
	}
	return user, err
}

func (uc *UserUsecase) DeleteUser(ctx context.Context, id int64) error {
	err := uc.repo.DeleteUser(ctx, id)
	if err != nil && stderrors.Is(err, ErrNoRowsUpdate) {
		return ErrConcurrentUpdate
	}
	return err
}
