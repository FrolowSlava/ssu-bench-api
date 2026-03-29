package repository

import (
	"context"
	"database/sql"
	"fmt"
	"ssu-bench-api/internal/models"
)

type BidRepository struct {
	db *sql.DB
}

func NewBidRepository(db *sql.DB) *BidRepository {
	return &BidRepository{db: db}
}

func (r *BidRepository) CreateBid(ctx context.Context, bid *models.Bid) error {
	query := `
	INSERT INTO bids (task_id, executor_id, amount, status, created_at, updated_at)
	VALUES ($1, $2, $3, $4, NOW(), NOW())
	RETURNING id, created_at, updated_at`
	return r.db.QueryRowContext(ctx, query,
		bid.TaskID, bid.ExecutorID, bid.Amount, bid.Status,
	).Scan(&bid.ID, &bid.CreatedAt, &bid.UpdatedAt)
}

func (r *BidRepository) GetBidByID(ctx context.Context, id int) (*models.Bid, error) {
	query := `SELECT id, task_id, executor_id, amount, status, created_at, updated_at
	FROM bids WHERE id = $1`
	var bid models.Bid
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&bid.ID, &bid.TaskID, &bid.ExecutorID, &bid.Amount,
		&bid.Status, &bid.CreatedAt, &bid.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrBidNotFound
		}
		return nil, fmt.Errorf("failed to get bid: %w", err)
	}
	return &bid, nil
}

func (r *BidRepository) GetBidsByTaskID(ctx context.Context, taskID int) ([]models.Bid, error) {
	query := `SELECT id, task_id, executor_id, amount, status, created_at, updated_at
	FROM bids WHERE task_id = $1 ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(ctx, query, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to query bids: %w", err)
	}
	defer rows.Close()
	var bids []models.Bid
	for rows.Next() {
		var b models.Bid
		if err := rows.Scan(&b.ID, &b.TaskID, &b.ExecutorID, &b.Amount,
			&b.Status, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan bid: %w", err)
		}
		bids = append(bids, b)
	}
	return bids, nil
}

// ищем отклики со статусом 'selected' ИЛИ 'completed'
func (r *BidRepository) GetSelectedBidForTask(ctx context.Context, taskID int) (*models.Bid, error) {
	query := `SELECT id, task_id, executor_id, amount, status, created_at, updated_at
	FROM bids WHERE task_id = $1 AND status IN ('selected', 'completed') LIMIT 1`
	var bid models.Bid
	err := r.db.QueryRowContext(ctx, query, taskID).Scan(
		&bid.ID, &bid.TaskID, &bid.ExecutorID, &bid.Amount,
		&bid.Status, &bid.CreatedAt, &bid.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get selected bid: %w", err)
	}
	return &bid, nil
}

func (r *BidRepository) CanExecutorBid(ctx context.Context, taskID, executorID int) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM bids WHERE task_id = $1 AND executor_id = $2`
	err := r.db.QueryRowContext(ctx, query, taskID, executorID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check bid: %w", err)
	}
	return count == 0, nil
}

func (r *BidRepository) UpdateBidStatus(ctx context.Context, id int, status models.BidStatus) error {
	query := `UPDATE bids SET status = $1, updated_at = NOW() WHERE id = $2`
	result, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update bid status: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return models.ErrBidNotFound
	}
	return nil
}

func (r *BidRepository) ExecutorHasBid(ctx context.Context, taskID, executorID int) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(
		SELECT 1 FROM bids WHERE task_id = $1 AND executor_id = $2
	)`
	err := r.db.QueryRowContext(ctx, query, taskID, executorID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check bid existence: %w", err)
	}
	return exists, nil
}
