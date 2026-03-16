package repository

import (
	"context"
	"database/sql"
	"fmt"
	"ssu-bench-api/internal/models"
)

type PaymentRepository struct {
	db *sql.DB
}

func NewPaymentRepository(db *sql.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) CreatePayment(ctx context.Context, payment *models.Payment) error {
	query := `
	INSERT INTO payments (task_id, from_user_id, to_user_id, amount, type, status, description, created_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
	RETURNING id, created_at`
	return r.db.QueryRowContext(ctx, query,
		payment.TaskID, payment.FromUserID, payment.ToUserID, payment.Amount,
		payment.Type, payment.Status, payment.Description,
	).Scan(&payment.ID, &payment.CreatedAt)
}

func (r *PaymentRepository) GetPaymentByID(ctx context.Context, id int) (*models.Payment, error) {
	query := `SELECT id, task_id, from_user_id, to_user_id, amount, type, status, description, created_at, completed_at
	FROM payments WHERE id = $1`
	var payment models.Payment
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&payment.ID, &payment.TaskID, &payment.FromUserID, &payment.ToUserID,
		&payment.Amount, &payment.Type, &payment.Status, &payment.Description,
		&payment.CreatedAt, &payment.CompletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("payment not found")
		}
		return nil, fmt.Errorf("failed to get payment: %w", err)
	}
	return &payment, nil
}

func (r *PaymentRepository) TransferPoints(ctx context.Context, fromID, toID int, amount float64) (*models.PaymentResult, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var fromBalance float64
	query := `SELECT balance FROM users WHERE id = $1 FOR UPDATE`
	err = tx.QueryRowContext(ctx, query, fromID).Scan(&fromBalance)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sender user not found")
		}
		return nil, fmt.Errorf("failed to get sender balance: %w", err)
	}

	if fromBalance < amount {
		return &models.PaymentResult{
			FromBalance: fromBalance,
			ToBalance:   0,
			Success:     false,
		}, nil
	}

	_, err = tx.ExecContext(ctx, `UPDATE users SET balance = balance - $1, updated_at = NOW() WHERE id = $2`, amount, fromID)
	if err != nil {
		return nil, fmt.Errorf("failed to deduct from sender: %w", err)
	}
	_, err = tx.ExecContext(ctx, `UPDATE users SET balance = balance + $1, updated_at = NOW() WHERE id = $2`, amount, toID)
	if err != nil {
		return nil, fmt.Errorf("failed to credit to receiver: %w", err)
	}

	var toBalance float64
	err = tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id = $1`, toID).Scan(&toBalance)
	if err != nil {
		return nil, fmt.Errorf("failed to get receiver balance: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &models.PaymentResult{
		FromBalance: fromBalance - amount,
		ToBalance:   toBalance,
		Success:     true,
	}, nil
}

func (r *PaymentRepository) CreatePaymentInTx(ctx context.Context, tx *sql.Tx, payment *models.Payment) error {
	query := `
	INSERT INTO payments (task_id, from_user_id, to_user_id, amount, type, status, description, created_at, completed_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	RETURNING id`
	return tx.QueryRowContext(ctx, query,
		payment.TaskID, payment.FromUserID, payment.ToUserID, payment.Amount,
		payment.Type, payment.Status, payment.Description, payment.CreatedAt, payment.CompletedAt,
	).Scan(&payment.ID)
}

func (r *PaymentRepository) GetPaymentsByUser(ctx context.Context, userID int, limit, offset int) ([]models.Payment, int, error) {
	query := `
	SELECT id, task_id, from_user_id, to_user_id, amount, type, status, description, created_at, completed_at
	FROM payments
	WHERE from_user_id = $1 OR to_user_id = $1
	ORDER BY created_at DESC
	LIMIT $2 OFFSET $3`
	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query payments: %w", err)
	}
	defer rows.Close()

	var payments []models.Payment
	for rows.Next() {
		var p models.Payment
		if err := rows.Scan(&p.ID, &p.TaskID, &p.FromUserID, &p.ToUserID,
			&p.Amount, &p.Type, &p.Status, &p.Description, &p.CreatedAt, &p.CompletedAt); err != nil {
			return nil, 0, fmt.Errorf("failed to scan payment: %w", err)
		}
		payments = append(payments, p)
	}

	var total int
	countQuery := `SELECT COUNT(*) FROM payments WHERE from_user_id = $1 OR to_user_id = $1`
	_ = r.db.QueryRowContext(ctx, countQuery, userID).Scan(&total)

	return payments, total, nil
}

// === НОВЫЙ МЕТОД ДЛЯ АДМИНА ===

// GetAllPayments возвращает все платежи системы с пагинацией
func (r *PaymentRepository) GetAllPayments(ctx context.Context, page, limit int) ([]models.Payment, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	query := `SELECT id, task_id, from_user_id, to_user_id, amount, type, status, description, created_at, completed_at
		FROM payments ORDER BY created_at DESC LIMIT $1 OFFSET $2`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query payments: %w", err)
	}
	defer rows.Close()

	var payments []models.Payment
	for rows.Next() {
		var p models.Payment
		if err := rows.Scan(&p.ID, &p.TaskID, &p.FromUserID, &p.ToUserID,
			&p.Amount, &p.Type, &p.Status, &p.Description, &p.CreatedAt, &p.CompletedAt); err != nil {
			return nil, 0, fmt.Errorf("failed to scan payment: %w", err)
		}
		payments = append(payments, p)
	}

	var total int
	countQuery := `SELECT COUNT(*) FROM payments`
	_ = r.db.QueryRowContext(ctx, countQuery).Scan(&total)

	return payments, total, nil
}
