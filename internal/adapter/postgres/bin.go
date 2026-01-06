package postgres

import (
	"context"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"

	"ecomap/internal/domain/bin"
)

type BinRepo struct {
	pool  *pgxpool.Pool
}

func NewBinRepo(pool *pgxpool.Pool) *BinRepo {
	return &BinRepo{pool:pool}
}

// func (r *BinRepo) GetByOwner(ctx context.Context, ownerID uint64) ([]*bin.Bin, error) {
// 	rows, err := r.pool.Query(ctx, `
// 		SELECT id, owner_id, qr_code, photo_url, lat, lon, address, landmark, status
// 		FROM bins
// 		WHERE owner_id = $1
// 	`, ownerID)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer rows.Close()

// 	var bins []*bin.Bin
// 	for rows.Next() {
// 		b := &bin.Bin{}
// 		if err := rows.Scan(&b.ID, &b.OwnerID, &b.QRCode, &b.PhotoURL, &b.Lat, &b.Lon, &b.Address, &b.Landmark, &b.Status); err != nil {
// 			return nil, err
// 		}
// 		bins = append(bins, b)
// 	}

// 	return bins, nil
// }

// Получение урны по ID
func (r *BinRepo) GetByID(ctx context.Context, id int64) (*bin.Bin, error) {
	b := &bin.Bin{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, owner_id, qr_code, photo_url, lat, lon, address, landmark, status
		FROM bins
		WHERE id = $1
	`, id).Scan(&b.ID, &b.OwnerID, &b.QRCode, &b.PhotoURL, &b.Lat, &b.Lon, &b.Address, &b.Landmark, &b.Status)

	if err != nil {
		return nil, err
	}
	return b, nil
}

// Обновление урны (например, смена статуса)
func (r *BinRepo) Update(ctx context.Context, b *bin.Bin) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE bins
		SET status=$1, photo_url=$2, address=$3, landmark=$4, lat=$5, lon=$6
		WHERE id=$7
	`, b.Status, b.PhotoURL, b.Address, b.Landmark, b.Lat, b.Lon, b.ID)
	return err
}


func (r *BinRepo) Create(ctx context.Context, b *bin.Bin) error {
    query := `
        INSERT INTO bins (
            owner_id, qr_code, photo_url,
            lat, lon, address, landmark
        )
        VALUES ($1,$2,$3,$4,$5,$6,$7)
        RETURNING id
    `

    return r.pool.QueryRow(
        ctx,
        query,
        b.OwnerID,
        b.QRCode,
        b.PhotoURL,
        b.Lat,
        b.Lon,
        b.Address,
        b.Landmark,
    ).Scan(&b.ID)
}

func (r *BinRepo) GetByQR(ctx context.Context,qr string) (*bin.Bin, error) {

    query := `
        SELECT id, owner_id, qr_code, photo_url,
               lat, lon, address, landmark, verified
        FROM bins
        WHERE qr_code = $1
    `

    b := &bin.Bin{}
    err := r.pool.QueryRow(ctx, query, qr).Scan(
        &b.ID,
        &b.OwnerID,
        &b.QRCode,
        &b.PhotoURL,
        &b.Lat,
        &b.Lon,
        &b.Address,
        &b.Landmark,
        &b.Verified,
    )

    if err == pgx.ErrNoRows {
        return nil, nil
    }

    return b, err
}

func (r *BinRepo) GetByOwner(ctx context.Context,ownerID int64) ([]*bin.Bin, error) {

    query := `
        SELECT id, owner_id, qr_code, photo_url,
               lat, lon, address, landmark, verified
        FROM bins
        WHERE owner_id = $1
    `

    rows, err := r.pool.Query(ctx, query, ownerID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var bins []*bin.Bin

    for rows.Next() {
        b := &bin.Bin{}
        rows.Scan(
            &b.ID,
            &b.OwnerID,
            &b.QRCode,
            &b.PhotoURL,
            &b.Lat,
            &b.Lon,
            &b.Address,
            &b.Landmark,
            &b.Verified,
        )
        bins = append(bins, b)
    }

    return bins, nil
}

func (r *BinRepo) Verify(ctx context.Context, binID int64) error {
    _, err := r.pool.Exec(
        ctx,
        `UPDATE bins SET verified = TRUE WHERE id = $1`,
        binID,
    )
    return err
}

