package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"ecomap/internal/domain/bin"
	"ecomap/internal/domain/workflow"
)

type WorkflowRepo struct{ pool *pgxpool.Pool }

func NewWorkflowRepo(pool *pgxpool.Pool) *WorkflowRepo { return &WorkflowRepo{pool: pool} }

func (r *WorkflowRepo) CreateIssue(ctx context.Context, telegramID int64, binID uint64, description string) (uint64, error) {
	var id uint64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO pickup_issues (user_id, bin_id, description)
		SELECT u.id, b.id, $3 FROM users u JOIN bins b ON b.owner_id = u.id
		WHERE u.telegram_id = $1 AND b.id = $2
		RETURNING id
	`, telegramID, binID, description).Scan(&id)
	return id, err
}

func (r *WorkflowRepo) ListUserIssues(ctx context.Context, telegramID int64) ([]workflow.Issue, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT i.id, i.bin_id, b.material, b.address, i.description, i.status, i.created_at
		FROM pickup_issues i JOIN users u ON u.id = i.user_id JOIN bins b ON b.id = i.bin_id
		WHERE u.telegram_id = $1 ORDER BY i.created_at DESC LIMIT 10
	`, telegramID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []workflow.Issue
	for rows.Next() {
		var item workflow.Issue
		if err := rows.Scan(&item.ID, &item.BinID, &item.Material, &item.Address, &item.Description, &item.Status, &item.CreatedAt); err != nil { return nil, err }
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *WorkflowRepo) ListOpenIssues(ctx context.Context) ([]workflow.Issue, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT i.id, i.bin_id, b.material, b.address, i.description, i.status, i.created_at
		FROM pickup_issues i JOIN bins b ON b.id = i.bin_id
		WHERE i.status <> 'resolved' ORDER BY i.created_at ASC LIMIT 50
	`)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []workflow.Issue
	for rows.Next() {
		var item workflow.Issue
		if err := rows.Scan(&item.ID, &item.BinID, &item.Material, &item.Address, &item.Description, &item.Status, &item.CreatedAt); err != nil { return nil, err }
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *WorkflowRepo) ResolveIssue(ctx context.Context, id uint64) error {
	_, err := r.pool.Exec(ctx, `UPDATE pickup_issues SET status = 'resolved', resolved_at = now() WHERE id = $1`, id)
	return err
}

func (r *WorkflowRepo) CreateBatch(ctx context.Context, collectorID int64, material string) (workflow.Batch, error) {
	var b workflow.Batch
	tempCode := fmt.Sprintf("TMP-%d-%d", collectorID, time.Now().UnixNano())
	err := r.pool.QueryRow(ctx, `
		INSERT INTO collection_batches (code, material, collector_telegram_id)
		VALUES ($1, $2, $3) RETURNING id, created_at
	`, tempCode, material, collectorID).Scan(&b.ID, &b.CreatedAt)
	if err != nil { return b, err }
	b.Code = fmt.Sprintf("ECO-%s-%05d", time.Now().Format("20060102"), b.ID)
	if _, err := r.pool.Exec(ctx, `UPDATE collection_batches SET code = $1 WHERE id = $2`, b.Code, b.ID); err != nil { return workflow.Batch{}, err }
	b.Material, b.Status = material, "open"
	return b, nil
}

func (r *WorkflowRepo) GetOpenBatch(ctx context.Context, collectorID int64) (*workflow.Batch, error) {
	var b workflow.Batch
	err := r.pool.QueryRow(ctx, `
		SELECT id, code, material, status, collected_weight_kg, COALESCE(accepted_weight_kg, 0),
			accepted_weight_kg IS NOT NULL, created_at
		FROM collection_batches WHERE collector_telegram_id = $1 AND status = 'open'
		ORDER BY created_at DESC LIMIT 1
	`, collectorID).Scan(&b.ID, &b.Code, &b.Material, &b.Status, &b.CollectedKG, &b.AcceptedKG, &b.HasAcceptedKG, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) { return nil, nil }
	if err != nil { return nil, err }
	return &b, nil
}

func (r *WorkflowRepo) RecordPickup(ctx context.Context, batchID uint64, collectorID int64, qr string, weight float64) (workflow.Batch, int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil { return workflow.Batch{}, 0, err }
	defer tx.Rollback(ctx)

	var binID uint64
	var ownerTelegramID int64
	var material string
	var batchMaterial string
	err = tx.QueryRow(ctx, `
		SELECT b.id, b.material, u.telegram_id, cb.material
		FROM bins b JOIN users u ON u.id = b.owner_id
		JOIN collection_batches cb ON cb.id = $1
		WHERE b.qr_code = $2 AND cb.collector_telegram_id = $3 AND cb.status = 'open'
		FOR UPDATE OF b, cb
	`, batchID, qr, collectorID).Scan(&binID, &material, &ownerTelegramID, &batchMaterial)
	if errors.Is(err, pgx.ErrNoRows) { return workflow.Batch{}, 0, errors.New("bin QR or active batch not found") }
	if err != nil { return workflow.Batch{}, 0, err }
	if material != batchMaterial { return workflow.Batch{}, 0, errors.New("bin material does not match batch") }
	if _, err := tx.Exec(ctx, `
		INSERT INTO collection_events (batch_id, bin_id, collector_telegram_id, weight_kg)
		VALUES ($1, $2, $3, $4)
	`, batchID, binID, collectorID, weight); err != nil {
		if isUniqueViolation(err) { return workflow.Batch{}, 0, errors.New("this bin is already recorded in the batch") }
		return workflow.Batch{}, 0, err
	}
	if _, err := tx.Exec(ctx, `UPDATE bins SET status = $1 WHERE id = $2`, bin.StatusEmpty, binID); err != nil { return workflow.Batch{}, 0, err }
	var b workflow.Batch
	err = tx.QueryRow(ctx, `
		UPDATE collection_batches SET collected_weight_kg = collected_weight_kg + $1
		WHERE id = $2 RETURNING id, code, material, status, collected_weight_kg, created_at
	`, weight, batchID).Scan(&b.ID, &b.Code, &b.Material, &b.Status, &b.CollectedKG, &b.CreatedAt)
	if err != nil { return workflow.Batch{}, 0, err }
	if err := tx.Commit(ctx); err != nil { return workflow.Batch{}, 0, err }
	return b, ownerTelegramID, nil
}

func (r *WorkflowRepo) FinishBatch(ctx context.Context, batchID uint64, collectorID int64) (workflow.Batch, error) {
	var b workflow.Batch
	err := r.pool.QueryRow(ctx, `
		UPDATE collection_batches SET status = 'submitted', submitted_at = now()
		WHERE id = $1 AND collector_telegram_id = $2 AND status = 'open'
		RETURNING id, code, material, status, collected_weight_kg, created_at
	`, batchID, collectorID).Scan(&b.ID, &b.Code, &b.Material, &b.Status, &b.CollectedKG, &b.CreatedAt)
	return b, err
}

func (r *WorkflowRepo) AcceptBatch(ctx context.Context, code string, recyclerID int64, weight float64) (workflow.Batch, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil { return workflow.Batch{}, err }
	defer tx.Rollback(ctx)
	var b workflow.Batch
	err = tx.QueryRow(ctx, `
		SELECT id, code, material, status, collected_weight_kg, created_at
		FROM collection_batches WHERE code = $1 FOR UPDATE
	`, code).Scan(&b.ID, &b.Code, &b.Material, &b.Status, &b.CollectedKG, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) { return workflow.Batch{}, errors.New("batch not found") }
	if err != nil { return workflow.Batch{}, err }
	if b.Status != "submitted" { return workflow.Batch{}, errors.New("batch is not submitted") }
	if weight > b.CollectedKG+0.0005 { return workflow.Batch{}, errors.New("accepted weight cannot exceed collected weight") }
	status := "accepted"
	if weight+0.0005 < b.CollectedKG { status = "partial" }
	if weight == 0 { status = "rejected" }
	err = tx.QueryRow(ctx, `
		UPDATE collection_batches SET status = $1, accepted_weight_kg = $2, accepted_by = $3, accepted_at = now()
		WHERE id = $4 RETURNING id, code, material, status, collected_weight_kg, accepted_weight_kg, true, created_at
	`, status, weight, recyclerID, b.ID).Scan(&b.ID, &b.Code, &b.Material, &b.Status, &b.CollectedKG, &b.AcceptedKG, &b.HasAcceptedKG, &b.CreatedAt)
	if err != nil { return workflow.Batch{}, err }
	if err := tx.Commit(ctx); err != nil { return workflow.Batch{}, err }
	return b, nil
}

func (r *WorkflowRepo) GetImpact(ctx context.Context, telegramID int64) ([]workflow.Impact, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT b.material,
			COALESCE(SUM(e.weight_kg * cb.accepted_weight_kg / NULLIF(cb.collected_weight_kg, 0)), 0)::float8
		FROM users u JOIN bins b ON b.owner_id = u.id
		JOIN collection_events e ON e.bin_id = b.id
		JOIN collection_batches cb ON cb.id = e.batch_id
		WHERE u.telegram_id = $1 AND cb.accepted_weight_kg IS NOT NULL
		GROUP BY b.material ORDER BY b.material
	`, telegramID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []workflow.Impact
	for rows.Next() {
		var item workflow.Impact
		if err := rows.Scan(&item.Material, &item.AcceptedKG); err != nil { return nil, err }
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *WorkflowRepo) ListUserDumps(ctx context.Context, telegramID int64) ([]workflow.DumpReport, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT d.id, d.address, d.description, d.workflow_status, d.created_at
		FROM dumps d JOIN users u ON u.id = d.user_id WHERE u.telegram_id = $1
		ORDER BY d.created_at DESC LIMIT 10
	`, telegramID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []workflow.DumpReport
	for rows.Next() {
		var item workflow.DumpReport
		if err := rows.Scan(&item.ID, &item.Address, &item.Description, &item.Status, &item.CreatedAt); err != nil { return nil, err }
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *WorkflowRepo) ListOpenDumps(ctx context.Context) ([]workflow.DumpReport, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, address, description, workflow_status, created_at FROM dumps
		WHERE workflow_status <> 'resolved' ORDER BY created_at ASC LIMIT 50
	`)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []workflow.DumpReport
	for rows.Next() {
		var item workflow.DumpReport
		if err := rows.Scan(&item.ID, &item.Address, &item.Description, &item.Status, &item.CreatedAt); err != nil { return nil, err }
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *WorkflowRepo) ResolveDump(ctx context.Context, id uint64) error {
	_, err := r.pool.Exec(ctx, `UPDATE dumps SET workflow_status = 'resolved' WHERE id = $1`, id)
	return err
}

func (r *WorkflowRepo) SetReminders(ctx context.Context, telegramID int64, enabled bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET reminders_enabled = $1 WHERE telegram_id = $2`, enabled, telegramID)
	return err
}

func (r *WorkflowRepo) ListReminderRecipients(ctx context.Context) ([]int64, error) {
	rows, err := r.pool.Query(ctx, `SELECT telegram_id FROM users WHERE reminders_enabled = TRUE ORDER BY id`)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil { return nil, err }
		out = append(out, id)
	}
	return out, rows.Err()
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
