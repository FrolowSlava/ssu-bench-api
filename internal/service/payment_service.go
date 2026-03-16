package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"ssu-bench-api/internal/models"
	"ssu-bench-api/internal/repository"
	"time"
)

type PaymentService struct {
	paymentRepo *repository.PaymentRepository
	taskRepo    *repository.TaskRepository
	bidRepo     *repository.BidRepository
	userRepo    *repository.UserRepository
	db          *sql.DB
}

func NewPaymentService(paymentRepo *repository.PaymentRepository, taskRepo *repository.TaskRepository, bidRepo *repository.BidRepository, userRepo *repository.UserRepository, db *sql.DB) *PaymentService {
	return &PaymentService{
		paymentRepo: paymentRepo,
		taskRepo:    taskRepo,
		bidRepo:     bidRepo,
		userRepo:    userRepo,
		db:          db,
	}
}

func (s *PaymentService) ProcessReward(ctx context.Context, taskID, customerID int) (*models.PaymentResult, error) {
	task, err := s.taskRepo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}
	if task.CustomerID != customerID {
		user, _ := s.userRepo.GetUserByID(ctx, customerID)
		if user == nil || user.Role != models.RoleAdmin {
			return nil, errors.New("only task owner can confirm completion")
		}
	}
	if task.Status != models.TaskStatusInProgress {
		return nil, fmt.Errorf("task must be in_progress to confirm, current status: %s", task.Status)
	}
	selectedBid, err := s.bidRepo.GetSelectedBidForTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get selected bid: %w", err)
	}
	if selectedBid == nil {
		return nil, errors.New("no selected bid found for task")
	}
	if selectedBid.Status != models.BidStatusCompleted {
		return nil, errors.New("executor has not marked task as completed")
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var customerBalance, executorBalance float64
	err = tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id = $1 FOR UPDATE`, customerID).Scan(&customerBalance)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("customer not found")
		}
		return nil, fmt.Errorf("failed to lock customer balance: %w", err)
	}
	err = tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id = $1 FOR UPDATE`, selectedBid.ExecutorID).Scan(&executorBalance)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("executor not found")
		}
		return nil, fmt.Errorf("failed to lock executor balance: %w", err)
	}

	amount := selectedBid.Amount
	if customerBalance < amount {
		return &models.PaymentResult{
			PaymentID:   0,
			FromBalance: customerBalance,
			ToBalance:   executorBalance,
			Success:     false,
			Error:       "insufficient balance",
		}, nil
	}

	_, err = tx.ExecContext(ctx, `UPDATE users SET balance = balance - $1, updated_at = NOW() WHERE id = $2`, amount, customerID)
	if err != nil {
		return nil, fmt.Errorf("failed to deduct from customer: %w", err)
	}
	_, err = tx.ExecContext(ctx, `UPDATE users SET balance = balance + $1, updated_at = NOW() WHERE id = $2`, amount, selectedBid.ExecutorID)
	if err != nil {
		return nil, fmt.Errorf("failed to credit executor: %w", err)
	}

	payment := &models.Payment{
		TaskID:      taskID,
		FromUserID:  customerID,
		ToUserID:    selectedBid.ExecutorID,
		Amount:      amount,
		Type:        models.PaymentTypeReward,
		Status:      "completed",
		Description: fmt.Sprintf("Reward for task #%d: %s", taskID, task.Title),
		CreatedAt:   time.Now(),
		CompletedAt: &[]time.Time{time.Now()}[0],
	}
	if err := s.paymentRepo.CreatePaymentInTx(ctx, tx, payment); err != nil {
		// Логируем ошибку, но не прерываем успешный перевод
	}

	_, err = tx.ExecContext(ctx, `UPDATE tasks SET status = 'completed', updated_at = NOW() WHERE id = $1`, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to update task status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &models.PaymentResult{
		PaymentID:   payment.ID,
		FromBalance: customerBalance - amount,
		ToBalance:   executorBalance + amount,
		Success:     true,
		Amount:      amount,
	}, nil
}

func (s *PaymentService) GetPaymentHistory(ctx context.Context, userID int, limit, offset int) ([]models.Payment, int, error) {
	return s.paymentRepo.GetPaymentsByUser(ctx, userID, limit, offset)
}

func (s *PaymentService) GetUserBalance(ctx context.Context, userID int) (float64, error) {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return 0, err
	}
	return user.Balance, nil
}

// === НОВЫЙ МЕТОД ДЛЯ АДМИНА ===

// GetAllPayments возвращает все платежи системы (для админа)
func (s *PaymentService) GetAllPayments(ctx context.Context, page, limit int) ([]models.Payment, int, error) {
	return s.paymentRepo.GetAllPayments(ctx, page, limit)
}
