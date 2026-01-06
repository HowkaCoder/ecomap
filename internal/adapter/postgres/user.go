package postgres

import (
	"context"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"

	"ecomap/internal/domain/user"
)

type UserRepo struct {
	pool  *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool:pool}
}

func (r *UserRepo) CreateUser(ctx context.Context, u *user.User) error {
    query := `
        INSERT INTO users (telegram_id, full_name, photo_url)
        VALUES ($1, $2, $3)
        RETURNING id
    `
    return r.pool.QueryRow(
        ctx,
        query,
        u.TelegramID,
        u.FullName,
        u.PhotoURL,
    ).Scan(&u.ID)
}




func (r *UserRepo) GetByTelegramID(ctx context.Context,id int64) (*user.User, error) {

    query := `
        SELECT id, telegram_id, full_name, photo_url, reputation
        FROM users
        WHERE telegram_id = $1
    `

    u := &user.User{}
    err := r.pool.QueryRow(ctx, query, id).Scan(
        &u.ID,
        &u.TelegramID,
        &u.FullName,
        &u.PhotoURL,
        &u.Reputation,
    )

    if err == pgx.ErrNoRows {
        return nil, nil
    }

    return u, err
}
