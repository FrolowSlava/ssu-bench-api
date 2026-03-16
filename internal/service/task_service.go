package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"ssu-bench-api/internal/models"
	"ssu-bench-api/internal/repository"
)

type TaskService struct {
	taskRepo *repository.TaskRepository
	bidRepo  *repository.BidRepository
	userRepo *repository.UserRepository
	db       *sql.DB
}

func NewTaskService(taskRepo *repository.TaskRepository, bidRepo *repository.BidRepository, userRepo *repository.UserRepository, db *sql.DB) *TaskService {
	return &TaskService{
		taskRepo: taskRepo,
		bidRepo:  bidRepo,
		userRepo: userRepo,
		db:       db,
	}
}

func (s *TaskService) CreateTask(ctx context.Context, customerID int, req *models.CreateTaskRequest) (*models.Task, error) {
	user, err := s.userRepo.GetUserByID(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("customer not found: %w", err)
	}
	if user.Role != models.RoleCustomer && user.Role != models.RoleAdmin {
		return nil, errors.New("only customers can create tasks")
	}
	task := &models.Task{
		Title:       req.Title,
		Description: req.Description,
		CustomerID:  customerID,
		Budget:      req.Budget,
		Status:      models.TaskStatusOpen,
	}
	if err := s.taskRepo.CreateTask(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}
	return task, nil
}

func (s *TaskService) GetTask(ctx context.Context, id int) (*models.Task, error) {
	task, err := s.taskRepo.GetTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}
	customer, err := s.userRepo.GetUserByID(ctx, task.CustomerID)
	if err == nil {
		task.CustomerUsername = customer.Username
	}
	selectedBid, _ := s.bidRepo.GetSelectedBidForTask(ctx, id)
	if selectedBid != nil {
		task.SelectedBidID = &selectedBid.ID
	}
	return task, nil
}

func (s *TaskService) ListTasks(ctx context.Context, query models.ListTasksQuery) ([]models.Task, int, error) {
	return s.taskRepo.GetTasks(ctx, query)
}

func (s *TaskService) SelectBid(ctx context.Context, taskID, customerID, bidID int) error {
	task, err := s.taskRepo.GetTaskByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}
	if task.CustomerID != customerID {
		return errors.New("only task owner can select bid")
	}
	if task.Status != models.TaskStatusOpen {
		return fmt.Errorf("can only select bid for open tasks, current status: %s", task.Status)
	}
	bid, err := s.bidRepo.GetBidByID(ctx, bidID)
	if err != nil {
		return fmt.Errorf("bid not found: %w", err)
	}
	if bid.TaskID != taskID {
		return errors.New("bid does not belong to this task")
	}
	if bid.Status != models.BidStatusPending {
		return fmt.Errorf("can only select pending bids, current status: %s", bid.Status)
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var currentStatus models.TaskStatus
	err = tx.QueryRowContext(ctx, `SELECT status FROM tasks WHERE id = $1 FOR UPDATE`, taskID).Scan(&currentStatus)
	if err != nil {
		return fmt.Errorf("failed to lock task: %w", err)
	}
	if currentStatus != models.TaskStatusOpen {
		return errors.New("task status changed, cannot select bid")
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE bids
		SET status = CASE
			WHEN id = $1 THEN 'selected'
			ELSE 'rejected'
		END,
		updated_at = NOW()
		WHERE task_id = $2 AND status = 'pending'
	`, bidID, taskID)
	if err != nil {
		return fmt.Errorf("failed to update bids: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return errors.New("no pending bids found for this task")
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE tasks
		SET status = 'in_progress', updated_at = NOW()
		WHERE id = $1
	`, taskID)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (s *TaskService) UpdateTaskStatus(ctx context.Context, taskID, userID int, newStatus models.TaskStatus) error {
	task, err := s.taskRepo.GetTaskByID(ctx, taskID)
	if err != nil {
		return err
	}
	if task.CustomerID != userID {
		user, _ := s.userRepo.GetUserByID(ctx, userID)
		if user == nil || user.Role != models.RoleAdmin {
			return errors.New("only task owner or admin can update status")
		}
	}
	switch newStatus {
	case models.TaskStatusCancelled:
		if task.Status == models.TaskStatusCompleted {
			return errors.New("cannot cancel completed task")
		}
		if task.Status != models.TaskStatusOpen && task.Status != models.TaskStatusInProgress {
			return fmt.Errorf("cannot cancel task with status: %s", task.Status)
		}
	case models.TaskStatusCompleted:
		if task.Status != models.TaskStatusInProgress {
			return errors.New("task must be in_progress to complete")
		}
		selectedBid, err := s.bidRepo.GetSelectedBidForTask(ctx, taskID)
		if err != nil {
			return err
		}
		if selectedBid == nil || selectedBid.Status != models.BidStatusCompleted {
			return errors.New("executor has not marked task as completed")
		}
	case models.TaskStatusInProgress:
		if task.Status != models.TaskStatusOpen {
			return fmt.Errorf("cannot transition from %s to in_progress", task.Status)
		}
	}
	return s.taskRepo.UpdateTaskStatus(ctx, taskID, newStatus)
}

func (s *TaskService) CanCancelTask(ctx context.Context, taskID int) (bool, error) {
	return s.taskRepo.CanCancelTask(ctx, taskID)
}

// === НОВЫЙ МЕТОД ДЛЯ АДМИНА ===

// GetAllTasks возвращает все задачи системы (для админа)
func (s *TaskService) GetAllTasks(ctx context.Context, page, limit int, statusFilter string) ([]models.Task, int, error) {
	query := models.ListTasksQuery{
		Status: statusFilter,
		Page:   page,
		Limit:  limit,
	}
	return s.taskRepo.GetTasks(ctx, query)
}
