package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ecomap/internal/domain/bin"
)

const binColumns = `id, owner_id, qr_code, photo_url, lat, lon, address, landmark, status, verified`

type BinRepo struct {
	pool *pgxpool.Pool
}

func NewBinRepo(pool *pgxpool.Pool) *BinRepo {
	return &BinRepo{pool: pool}
}

func scanBin(row pgx.Row) (*bin.Bin, error) {
	b := &bin.Bin{}
	err := row.Scan(&b.ID, &b.OwnerID, &b.QRCode, &b.PhotoURL, &b.Lat, &b.Lon, &b.Address, &b.Landmark, &b.Status, &b.Verified)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (r *BinRepo) Create(ctx context.Context, b *bin.Bin) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO bins (owner_id, qr_code, photo_url, lat, lon, address, landmark, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`, b.OwnerID, b.QRCode, b.PhotoURL, b.Lat, b.Lon, b.Address, b.Landmark, b.Status).Scan(&b.ID)
}

func (r *BinRepo) GetByID(ctx context.Context, id uint64) (*bin.Bin, error) {
	b, err := scanBin(r.pool.QueryRow(ctx, `SELECT `+binColumns+` FROM bins WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return b, err
}

func (r *BinRepo) GetByQR(ctx context.Context, qr string) (*bin.Bin, error) {
	b, err := scanBin(r.pool.QueryRow(ctx, `SELECT `+binColumns+` FROM bins WHERE qr_code = $1`, qr))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return b, err
}

func (r *BinRepo) GetByOwner(ctx context.Context, ownerID uint64) ([]*bin.Bin, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+binColumns+` FROM bins WHERE owner_id = $1 ORDER BY id`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bins []*bin.Bin
	for rows.Next() {
		b, err := scanBin(rows)
		if err != nil {
			return nil, err
		}
		bins = append(bins, b)
	}
	return bins, rows.Err()
}

func (r *BinRepo) Update(ctx context.Context, b *bin.Bin) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE bins
		SET status = $1, photo_url = $2, address = $3, landmark = $4, lat = $5, lon = $6
		WHERE id = $7
	`, b.Status, b.PhotoURL, b.Address, b.Landmark, b.Lat, b.Lon, b.ID)
	return err
}

func (r *BinRepo) Verify(ctx context.Context, binID uint64) error {
	_, err := r.pool.Exec(ctx, `UPDATE bins SET verified = TRUE WHERE id = $1`, binID)
	return err
}
