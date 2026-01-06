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
	return &DumpRepo{pool:pool}
}

func (dp *DumpRepo) CreateDump(ctx context.Context , dump *dump.Dump) error {
	query  := ` 
		INSERT INTO dumps (user_id , photo_url , lat , lon , address , landmark , description , status) 
		VALUES ($1 , $2 , $3 , $4 , $5 , $6 , $7 , $8)
		RETURNING id
	`

	return dp.pool.QueryRow(
		ctx , 
		query , 
		dump.UserID , 
		dump.PhotoURL , 
		dump.Lat,
		dump.Lon,
		dump.Address,
		dump.Landmark,
		dump.Description,
		dump.Status,
	).Scan(&dump.ID)
}