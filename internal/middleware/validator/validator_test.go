package validator_test

import (
	"testing"

	pb "github.com/go-kratos/kratos-layout-monolith/api/user/v1"
	"github.com/go-kratos/kratos-layout-monolith/internal/middleware/validator"
)

func TestRegisterRequest_Validate_Success(t *testing.T) {
	req := &pb.RegisterRequest{
		Username: "testuser",
		Password: "password123",
		Email:    "test@example.com",
		Phone:    "13800138000",
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestRegisterRequest_Validate_Failure(t *testing.T) {
	tests := []struct {
		name string
		req  *pb.RegisterRequest
	}{
		{
			name: "empty username",
			req: &pb.RegisterRequest{
				Username: "",
				Password: "password123",
				Email:    "test@example.com",
			},
		},
		{
			name: "password too short",
			req: &pb.RegisterRequest{
				Username: "testuser",
				Password: "123",
				Email:    "test@example.com",
			},
		},
		{
			name: "invalid email",
			req: &pb.RegisterRequest{
				Username: "testuser",
				Password: "password123",
				Email:    "not-an-email",
			},
		},
		{
			name: "invalid phone",
			req: &pb.RegisterRequest{
				Username: "testuser",
				Password: "password123",
				Email:    "test@example.com",
				Phone:    "abc",
			},
		},
		{
			name: "username with invalid chars",
			req: &pb.RegisterRequest{
				Username: "test user!",
				Password: "password123",
				Email:    "test@example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if err == nil {
				t.Fatalf("expected validation error, got nil")
			}
		})
	}
}

func TestLoginRequest_Validate(t *testing.T) {
	req := &pb.LoginRequest{
		Username: "",
		Password: "",
	}
	if err := req.Validate(); err == nil {
		t.Fatal("expected validation error for empty login request")
	}
}

func TestGetUserRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		id      int64
		wantErr bool
	}{
		{"valid id", 1, false},
		{"zero id", 0, true},
		{"negative id", -1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &pb.GetUserRequest{Id: tt.id}
			err := req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetUserRequest.Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestListUsersRequest_Validate(t *testing.T) {
	tests := []struct {
		name     string
		page     int32
		pageSize int32
		wantErr  bool
	}{
		{"valid defaults", 1, 10, false},
		{"zero page", 0, 10, true},
		{"page_size too large", 1, 200, true},
		{"negative page size", 1, -1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &pb.ListUsersRequest{Page: tt.page, PageSize: tt.pageSize}
			err := req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ListUsersRequest.Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpdateUserRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     *pb.UpdateUserRequest
		wantErr bool
	}{
		{
			name: "valid",
			req: &pb.UpdateUserRequest{
				Id:   1,
				User: &pb.User{Username: "updated"},
			},
			wantErr: false,
		},
		{
			name: "zero id",
			req: &pb.UpdateUserRequest{
				Id:   0,
				User: &pb.User{},
			},
			wantErr: true,
		},
		{
			name: "nil user",
			req: &pb.UpdateUserRequest{
				Id:   1,
				User: nil,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateUserRequest.Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeleteUserRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		id      int64
		wantErr bool
	}{
		{"valid", 1, false},
		{"zero", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &pb.DeleteUserRequest{Id: tt.id}
			err := req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteUserRequest.Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestMiddlewareServer_ValidateReq(t *testing.T) {
	// Valid request should pass through middleware
	validReq := &pb.RegisterRequest{
		Username: "testuser",
		Password: "password123",
		Email:    "test@example.com",
	}
	err := validator.ValidateReq(validReq)
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}

	// Invalid request should be rejected
	invalidReq := &pb.RegisterRequest{
		Username: "",
		Password: "short",
		Email:    "bad",
	}
	err = validator.ValidateReq(invalidReq)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}
