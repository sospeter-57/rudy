package models

import "fmt"

type ErrorKind string

const (
	KindNotFound     ErrorKind = "not_found"
	KindUnauthorized ErrorKind = "unauthorized"
	KindForbidden    ErrorKind = "forbidden"
	KindConflict     ErrorKind = "conflict"
	KindValidation   ErrorKind = "validation"
	KindInternal     ErrorKind = "internal"
)

type AppError struct {
	Kind    ErrorKind
	Message string
}

func (e *AppError) Error() string {
	return e.Message
}

func NewError(kind ErrorKind, message string) *AppError {
	return &AppError{Kind: kind, Message: message}
}

func NotFound(message string) *AppError {
	return NewError(KindNotFound, message)
}

func Unauthorized(message string) *AppError {
	return NewError(KindUnauthorized, message)
}

func Forbidden(message string) *AppError {
	return NewError(KindForbidden, message)
}

func Conflict(message string) *AppError {
	return NewError(KindConflict, message)
}

func Validation(format string, args ...any) *AppError {
	return NewError(KindValidation, fmt.Sprintf(format, args...))
}
