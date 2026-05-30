package errors

import (
	"fmt"
	"net/http"
)

// Code represents a machine-readable error classification.
type Code string

const (
	CodeNotFound     Code = "NOT_FOUND"
	CodeUnauthorized Code = "UNAUTHORIZED"
	CodeForbidden    Code = "FORBIDDEN"
	CodeValidation   Code = "VALIDATION_ERROR"
	CodeConflict     Code = "CONFLICT"
	CodeInternal     Code = "INTERNAL_ERROR"
	CodeBadRequest   Code = "BAD_REQUEST"
)

// AppError is a structured application error that carries a machine-readable code,
// a human-readable message, an optional detail string, and an optional wrapped error.
type AppError struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
	Err     error  `json:"-"`
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error for use with errors.Is and errors.As.
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewNotFound creates a NOT_FOUND error.
func NewNotFound(msg string) *AppError {
	return &AppError{Code: CodeNotFound, Message: msg}
}

// NewUnauthorized creates an UNAUTHORIZED error.
func NewUnauthorized(msg string) *AppError {
	return &AppError{Code: CodeUnauthorized, Message: msg}
}

// NewForbidden creates a FORBIDDEN error.
func NewForbidden(msg string) *AppError {
	return &AppError{Code: CodeForbidden, Message: msg}
}

// NewValidation creates a VALIDATION_ERROR error.
func NewValidation(msg string) *AppError {
	return &AppError{Code: CodeValidation, Message: msg}
}

// NewConflict creates a CONFLICT error.
func NewConflict(msg string) *AppError {
	return &AppError{Code: CodeConflict, Message: msg}
}

// NewInternal creates an INTERNAL_ERROR error.
// It accepts an optional underlying error for wrapping.
func NewInternal(msg string, err error) *AppError {
	return &AppError{Code: CodeInternal, Message: msg, Err: err}
}

// NewBadRequest creates a BAD_REQUEST error.
func NewBadRequest(msg string) *AppError {
	return &AppError{Code: CodeBadRequest, Message: msg}
}

// WithDetail returns a copy of the AppError with the detail field set.
func (e *AppError) WithDetail(detail string) *AppError {
	e.Detail = detail
	return e
}

// WithErr returns a copy of the AppError with the wrapped error set.
func (e *AppError) WithErr(err error) *AppError {
	e.Err = err
	return e
}

// HTTPStatus maps an AppError's Code to the corresponding HTTP status code.
func HTTPStatus(err *AppError) int {
	switch err.Code {
	case CodeNotFound:
		return http.StatusNotFound
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeValidation:
		return http.StatusUnprocessableEntity
	case CodeConflict:
		return http.StatusConflict
	case CodeBadRequest:
		return http.StatusBadRequest
	case CodeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}
