package repository

import (
	"context"
	"database/sql"
	"fmt"
	"ssu-bench-api/internal/models"
)

type TaskRepository struct {
	db *sql.DB
}

func NewTaskRepository(db *sql.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

// CreateTask создаёт новую задачу в базе данных
func (r *TaskRepository) CreateTask(ctx context.Context, task *models.Task) error {
	query := `
		INSERT INTO tasks (title, description, customer_id, budget, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		RETURNING id, created_at, updated_at`

	return r.db.QueryRowContext(ctx, query,
		task.Title, task.Description, task.CustomerID, task.Budget, task.Status,
	).Scan(&task.ID, &task.CreatedAt, &task.UpdatedAt)
}

// GetTaskByID возвращает задачу по ID
func (r *TaskRepository) GetTaskByID(ctx context.Context, id int) (*models.Task, error) {
	query := `SELECT id, title, description, customer_id, budget, status, created_at, updated_at 
			  FROM tasks WHERE id = $1`

	var task models.Task
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&task.ID, &task.Title, &task.Description, &task.CustomerID,
		&task.Budget, &task.Status, &task.CreatedAt, &task.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrTaskNotFound
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}
	return &task, nil
}

// GetTasks возвращает список задач с фильтрами и пагинацией
// Возвращает: задачи, общее количество, ошибку
func (r *TaskRepository) GetTasks(ctx context.Context, query models.ListTasksQuery) ([]models.Task, int, error) {
	// Устанавливаем значения по умолчанию для пагинации
	if query.Page < 1 {
		query.Page = 1
	}
	if query.Limit < 1 || query.Limit > 100 {
		query.Limit = 20
	}

	// Базовые запросы
	baseQuery := `SELECT id, title, description, customer_id, budget, status, created_at, updated_at FROM tasks WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM tasks WHERE 1=1`

	args := []interface{}{}

	// Фильтр по статусу
	if query.Status != "" {
		baseQuery += " AND status = $" + fmt.Sprintf("%d", len(args)+1)
		countQuery += " AND status = $" + fmt.Sprintf("%d", len(args)+1)
		args = append(args, query.Status)
	}

	// Фильтр по заказчику
	if query.Customer > 0 {
		baseQuery += " AND customer_id = $" + fmt.Sprintf("%d", len(args)+1)
		countQuery += " AND customer_id = $" + fmt.Sprintf("%d", len(args)+1)
		args = append(args, query.Customer)
	}

	// Пагинация (аргументы добавляем в конце)
	baseQuery += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, query.Limit, (query.Page-1)*query.Limit)

	// Выполняем основной запрос
	rows, err := r.db.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.CustomerID,
			&t.Budget, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("failed to scan task: %w", err)
		}
		tasks = append(tasks, t)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating tasks: %w", err)
	}

	// Выполняем COUNT с теми же фильтрами (без пагинации)
	// Аргументы для countQuery — это все, кроме последних двух (LIMIT/OFFSET)
	countArgs := args[:len(args)-2]
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count tasks: %w", err)
	}

	return tasks, total, nil
}

// UpdateTaskStatus обновляет статус задачи
func (r *TaskRepository) UpdateTaskStatus(ctx context.Context, id int, status models.TaskStatus) error {
	query := `UPDATE tasks SET status = $1, updated_at = NOW() WHERE id = $2`
	result, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return models.ErrTaskNotFound
	}
	return nil
}

// CanCancelTask проверяет, можно ли отменить задачу (нельзя отменить выполненную)
func (r *TaskRepository) CanCancelTask(ctx context.Context, id int) (bool, error) {
	var status models.TaskStatus
	query := `SELECT status FROM tasks WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(&status)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, models.ErrTaskNotFound
		}
		return false, err
	}

	// Нельзя отменить выполненную задачу
	return status != models.TaskStatusCompleted, nil
}

// SetSelectedBid атомарно выбирает одну заявку и отклоняет остальные
// Возвращает ошибку, если не удалось обновить
func (r *TaskRepository) SetSelectedBid(ctx context.Context, taskID, bidID int) error {
	// Используем константы из models для безопасности типов
	selectedStatus := string(models.BidStatusSelected)
	rejectedStatus := string(models.BidStatusRejected)
	pendingStatus := string(models.BidStatusPending)

	query := fmt.Sprintf(`
		UPDATE bids 
		SET status = CASE 
			WHEN id = $1 THEN '%s' 
			ELSE '%s' 
		END,
		updated_at = NOW()
		WHERE task_id = $2 AND status = '%s'`, selectedStatus, rejectedStatus, pendingStatus)

	_, err := r.db.ExecContext(ctx, query, bidID, taskID)
	if err != nil {
		return fmt.Errorf("failed to select bid: %w", err)
	}
	return nil
}
