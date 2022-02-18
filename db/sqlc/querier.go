// Code generated by sqlc. DO NOT EDIT.

package db

import (
	"context"
)

type Querier interface {
	CheckUserDir(ctx context.Context, arg CheckUserDirParams) (Directory, error)
	CreateUser(ctx context.Context, arg CreateUserParams) (User, error)
	CreateUserDir(ctx context.Context, arg CreateUserDirParams) (Directory, error)
	DeleteUserDir(ctx context.Context, arg DeleteUserDirParams) error
	GetUser(ctx context.Context, username string) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	GetUserDirs(ctx context.Context, userID int64) ([]Directory, error)
}

var _ Querier = (*Queries)(nil)
