package domain

import "errors"

var (
	ErrNotFound            = errors.New("resource not found")
	ErrAlreadyExists       = errors.New("resource already exists")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInvalidInput        = errors.New("invalid input")
	ErrForbidden           = errors.New("forbidden")
	ErrIdempotencyNotFound = errors.New("idempotency key not found")
)
