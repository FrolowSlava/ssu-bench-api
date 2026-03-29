package models

import "errors"

// Стандартные ошибки приложения
var (
	ErrUserNotFound        = errors.New("user not found")
	ErrTaskNotFound        = errors.New("task not found")
	ErrBidNotFound         = errors.New("bid not found")
	ErrPaymentNotFound     = errors.New("payment not found")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrForbidden           = errors.New("forbidden")
	ErrInvalidRequest      = errors.New("invalid request")
	ErrConflict            = errors.New("conflict")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrInvalidStatus       = errors.New("invalid status transition")
	ErrAlreadyExists       = errors.New("resource already exists")
	ErrCannotBlockAdmin    = errors.New("cannot block admin user")
	ErrOnlyTaskOwner       = errors.New("only task owner can perform this action")
	ErrOnlyExecutor        = errors.New("only executor can perform this action")
	ErrOnlyCustomer        = errors.New("only customer can perform this action")
)
