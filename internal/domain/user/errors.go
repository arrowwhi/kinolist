package user

import "errors"

var (
	ErrNotFound        = errors.New("user not found")
	ErrInvalidTelegram = errors.New("invalid telegram identity")
)
