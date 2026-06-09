package user

import "context"

type Repository interface {
	Find(ctx context.Context, id string) error
}
