package validator

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-playground/validator/v10"
)

// ValidationError is returned when struct validation fails.
type ValidationError struct {
	Errors []*FieldError
}

// FieldError holds details about a single field validation failure.
type FieldError struct {
	Field string `json:"field"`
	Tag   string `json:"tag"`
	Param string `json:"param,omitempty"`
	Value any    `json:"value,omitempty"`
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Errors))
	for _, f := range e.Errors {
		msg := f.Field
		if f.Param != "" {
			msg += fmt.Sprintf("(%s=%s)", f.Tag, f.Param)
		}
		parts = append(parts, msg)
	}
	return strings.Join(parts, "; ")
}

// FromError converts a validator error to ValidationError.
func FromError(err error) *ValidationError {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		fields := make([]*FieldError, 0, len(ve))
		for _, fe := range ve {
			fields = append(fields, &FieldError{
				Field: fe.Field(),
				Tag:   fe.Tag(),
				Param: fe.Param(),
				Value: fe.Value(),
			})
		}
		return &ValidationError{Errors: fields}
	}
	return nil
}

// ValidateReq checks if req implements Validate() error and calls it.
// Returns ValidationError if validation fails.
func ValidateReq(req interface{}) error {
	type validator interface{ Validate() error }
	if v, ok := req.(validator); ok {
		if err := v.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Server returns a Kratos middleware that calls Validate() on requests
// that implement the Validate() error interface.
func Server() middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if err := ValidateReq(req); err != nil {
				return nil, err
			}
			return next(ctx, req)
		}
	}
}
