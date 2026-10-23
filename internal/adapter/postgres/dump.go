package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"ecomap/internal/domain/dump"
)

type DumpRepo struct {
	pool *pgxpool.Pool
}

func NewDumpRepo(pool *pgxpool.Pool) *DumpRepo {
	return &DumpRepo{pool: pool}
}

func (r *DumpRepo) CreateDump(ctx context.Context, d *dump.Dump) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO dumps (user_id, photo_url, lat, lon, address, landmark, description, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`, d.UserID, d.PhotoURL, d.Lat, d.Lon, d.Address, d.Landmark, d.Description, d.Status).Scan(&d.ID)
}
