package usecase

import "errors"

var (
	ErrAlreadyRegistered = errors.New("user already registered")
	ErrNotRegistered     = errors.New("user not registered")
	ErrBinNotFound       = errors.New("bin not found")
	ErrNotOwner          = errors.New("bin belongs to another user")
	ErrUnknownStatus     = errors.New("unknown bin status")
)
