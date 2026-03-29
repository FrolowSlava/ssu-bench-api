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
	taskRepo    *repository.TaskRepository
	bidRepo     *repository.BidRepository
	userRepo    *repository.UserRepository
	paymentRepo *repository.PaymentRepository
	db          *sql.DB
}

// Обновлённый конструктор
func NewTaskService(
	taskRepo *repository.TaskRepository,
	bidRepo *repository.BidRepository,
	userRepo *repository.UserRepository,
	paymentRepo *repository.PaymentRepository,
	db *sql.DB,
) *TaskService {
	return &TaskService{
		taskRepo:    taskRepo,
		bidRepo:     bidRepo,
		userRepo:    userRepo,
		paymentRepo: paymentRepo,
		db:          db,
	}
}

func (s *TaskService) CreateTask(ctx context.Context, customerID int, req *models.CreateTaskRequest) (*models.Task, error) {
	user, err := s.userRepo.GetUserByID(ctx, customerID)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, models.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get customer: %w", err)
	}
	if user.Role != models.RoleCustomer && user.Role != models.RoleAdmin {
		return nil, models.ErrOnlyCustomer
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
		if errors.Is(err, models.ErrTaskNotFound) {
			return nil, models.ErrTaskNotFound
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
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
		if errors.Is(err, models.ErrTaskNotFound) {
			return models.ErrTaskNotFound
		}
		return fmt.Errorf("failed to get task: %w", err)
	}
	if task.CustomerID != customerID {
		return models.ErrOnlyTaskOwner
	}
	if task.Status != models.TaskStatusOpen {
		return fmt.Errorf("can only select bid for open tasks, current status: %s", task.Status)
	}
	bid, err := s.bidRepo.GetBidByID(ctx, bidID)
	if err != nil {
		if errors.Is(err, models.ErrBidNotFound) {
			return models.ErrBidNotFound
		}
		return fmt.Errorf("failed to get bid: %w", err)
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
		if errors.Is(err, models.ErrTaskNotFound) {
			return models.ErrTaskNotFound
		}
		return fmt.Errorf("failed to get task: %w", err)
	}
	if task.CustomerID != userID {
		user, _ := s.userRepo.GetUserByID(ctx, userID)
		if user == nil || user.Role != models.RoleAdmin {
			return models.ErrOnlyTaskOwner
		}
	}
	switch newStatus {
	case models.TaskStatusCancelled:
		return errors.New("use CancelTask method for cancellation")
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
		return errors.New("use ConfirmTaskCompletion method to finish the task")
	case models.TaskStatusInProgress:
		if task.Status != models.TaskStatusOpen {
			return fmt.Errorf("cannot transition from %s to in_progress", task.Status)
		}
	}
	return s.taskRepo.UpdateTaskStatus(ctx, taskID, newStatus)
}

func (s *TaskService) CanCancelTask(ctx context.Context, taskID int) (bool, error) {
	canCancel, err := s.taskRepo.CanCancelTask(ctx, taskID)
	if err != nil {
		if errors.Is(err, models.ErrTaskNotFound) {
			return false, models.ErrTaskNotFound
		}
		return false, err
	}
	return canCancel, nil
}

// НОВЫЙ МЕТОД: Подтверждение выполнения заказчиком или администратором
func (s *TaskService) ConfirmTaskCompletion(ctx context.Context, taskID, userID int) error {
	task, err := s.taskRepo.GetTaskByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, models.ErrTaskNotFound) {
			return models.ErrTaskNotFound
		}
		return fmt.Errorf("failed to get task: %w", err)
	}

	// Проверяем, что пользователь - владелец или админ
	if task.CustomerID != userID {
		user, _ := s.userRepo.GetUserByID(ctx, userID)
		if user == nil || user.Role != models.RoleAdmin {
			return models.ErrOnlyTaskOwner
		}
		// Если пользователь - админ, он может подтвердить, но деньги списываются с заказчика.
	}
	// Если userID == task.CustomerID, то пользователь - владелец.

	if task.Status != models.TaskStatusInProgress {
		return fmt.Errorf("task must be in_progress to confirm, current status: %s", task.Status)
	}

	selectedBid, err := s.bidRepo.GetSelectedBidForTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("failed to get selected bid: %w", err)
	}
	if selectedBid == nil {
		return errors.New("no selected bid found for task")
	}
	if selectedBid.Status != models.BidStatusCompleted {
		return errors.New("executor has not marked task as completed")
	}

	// Атомарная транзакция: сначала проверка баланса, потом перевод и обновление статуса
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// ВАЖНО: Проверяем баланс и списываем с task.CustomerID (владельца задачи), а не с userID (который может быть админом)!
	var customerBalance float64
	err = tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id = $1 FOR UPDATE`, task.CustomerID).Scan(&customerBalance)
	if err != nil {
		if err == sql.ErrNoRows {
			return models.ErrUserNotFound
		}
		return fmt.Errorf("failed to lock customer balance: %w", err)
	}

	if customerBalance < selectedBid.Amount {
		return models.ErrInsufficientBalance
	}

	// Списываем у заказчика (task.CustomerID)
	_, err = tx.ExecContext(ctx, `UPDATE users SET balance = balance - $1, updated_at = NOW() WHERE id = $2`, selectedBid.Amount, task.CustomerID)
	if err != nil {
		return fmt.Errorf("failed to deduct from customer: %w", err)
	}

	// Начисляем исполнителю (selectedBid.ExecutorID)
	_, err = tx.ExecContext(ctx, `UPDATE users SET balance = balance + $1, updated_at = NOW() WHERE id = $2`, selectedBid.Amount, selectedBid.ExecutorID)
	if err != nil {
		return fmt.Errorf("failed to credit executor: %w", err)
	}

	// Создаём запись о платеже
	payment := &models.Payment{
		TaskID:      taskID,
		FromUserID:  task.CustomerID,        // Отправитель - заказчик
		ToUserID:    selectedBid.ExecutorID, // Получатель - исполнитель
		Amount:      selectedBid.Amount,
		Type:        models.PaymentTypeReward,
		Status:      "completed",
		Description: fmt.Sprintf("Reward for task #%d: %s", taskID, task.Title),
	}
	// Используем CreatePaymentInTx, передавая транзакцию
	if err := s.paymentRepo.CreatePaymentInTx(ctx, tx, payment); err != nil {
		// Не прерываем транзакцию из-за ошибки создания платежа, если сам перевод прошёл
	}

	// Только после успешного перевода обновляем статус задачи
	_, err = tx.ExecContext(ctx, `UPDATE tasks SET status = 'completed', updated_at = NOW() WHERE id = $1`, taskID)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// НОВЫЙ МЕТОД: Отмена задачи (с проверкой)
func (s *TaskService) CancelTask(ctx context.Context, taskID, userID int) error {
	// Проверяем, можно ли отменить задачу (не completed)
	canCancel, err := s.taskRepo.CanCancelTask(ctx, taskID)
	if err != nil {
		if errors.Is(err, models.ErrTaskNotFound) {
			return models.ErrTaskNotFound
		}
		return err // task not found
	}
	if !canCancel {
		return errors.New("cannot cancel a completed task")
	}

	task, err := s.taskRepo.GetTaskByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, models.ErrTaskNotFound) {
			return models.ErrTaskNotFound
		}
		return err
	}
	// Проверяем, что пользователь - владелец или админ
	if task.CustomerID != userID {
		user, _ := s.userRepo.GetUserByID(ctx, userID)
		if user == nil || user.Role != models.RoleAdmin {
			return models.ErrOnlyTaskOwner
		}
	}

	// Обновляем статус на cancelled
	return s.taskRepo.UpdateTaskStatus(ctx, taskID, models.TaskStatusCancelled)
}

// === НОВЫЙ МЕТОД ДЛЯ АДМИНА ===
func (s *TaskService) GetAllTasks(ctx context.Context, page, limit int, statusFilter string) ([]models.Task, int, error) {
	query := models.ListTasksQuery{
		Status: statusFilter,
		Page:   page,
		Limit:  limit,
	}
	return s.taskRepo.GetTasks(ctx, query)
}
