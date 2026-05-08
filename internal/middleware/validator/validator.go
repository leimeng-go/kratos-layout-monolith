package validator

import (
	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
}

// Validate validates a struct with validation tags.
//
// Supported tags: required, email, min, max, len, range, oneof, gt, gte, lt, lte, eq, ne, url, etc.
//
// Example:
//
//	type RegisterRequest struct {
//	    Username string `json:"username" validate:"required,min=3,max=32"`
//	    Email    string `json:"email" validate:"required,email"`
//	    Age      int    `json:"age" validate:"gte=1,lte=120"`
//	    Status   int    `json:"status" validate:"oneof=0 1 2"`
//	    Phone    string `json:"phone" validate:"required,len=11"`
//	    Password string `json:"password" validate:"required,min=8,max=32"`
//	}
func Validate(v any) error {
	return validate.Struct(v)
}

// RegisterValidation registers a custom validation tag.
func RegisterValidation(tag string, fn validator.Func) error {
	return validate.RegisterValidation(tag, fn)
}
