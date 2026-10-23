package user

import "context"

type UserRepository interface {
	CreateUser(ctx context.Context, user *User) error
	GetByTelegramID(ctx context.Context, id int64) (*User, error)
}
