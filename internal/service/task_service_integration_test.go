// Покрывают критические сценарии:
// 1. Успешное подтверждение (транзакция)
// 2. Недостаточно баллов (откат)
// 3. Неверный статус задачи
// 4. Не владелец подтверждает
// 5. Отмена выполненной задачи (запрещено)
// 6. Успешная отмена задачи
// 7. Создание задачи не заказчиком (запрещено)
// 8. Выбор отклика не владельцем (запрещено)
// 9. Создание отклика не исполнителем (запрещено)
// 10. Выбор отклика для не open задачи (запрещено)
// 11. Дублирование отклика (запрещено)
// 12. Завершение не выбранного отклика (запрещено)
// 13. Подтверждение администратором (разрешено)
package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"ssu-bench-api/internal/models"
	"ssu-bench-api/internal/repository"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	// Загружаем .env.test из текущей директории (internal/service/)
	if err := godotenv.Load(".env.test"); err != nil {
		log.Printf("[WARN] Could not load .env.test: %v", err)
		// Если файл не найден, пытаемся использовать переменные окружения системы
	}

	// Получаем переменные окружения
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "password")
	dbname := getEnv("DB_NAME", "ssubench_test")
	sslmode := getEnv("DB_SSLMODE", "disable")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)

	var err error
	testDB, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to open test DB connection: %v", err)
	}
	defer testDB.Close()

	// Проверяем соединение
	if err := testDB.Ping(); err != nil {
		log.Fatalf("Failed to ping test DB '%s': %v", dbname, err)
	}

	code := m.Run()
	os.Exit(code)
}

// Вспомогательная функция для получения переменной окружения со значением по умолчанию
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// setupTestUser создает пользователя в тестовой БД и возвращает его.
// ИСПРАВЛЕНИЕ: Делаем уникальными и email, и username для предотвращения конфликтов
func setupTestUser(t *testing.T, username, email, role string) *models.User {
	userRepo := repository.NewUserRepository(testDB)
	userService := NewUserService(userRepo)

	// Добавляем уникальный суффикс на основе времени (наносекунды)
	uniqueSuffix := fmt.Sprintf("_%d", time.Now().UnixNano())
	uniqueEmail := fmt.Sprintf("%s%s", email, uniqueSuffix)
	uniqueUsername := fmt.Sprintf("%s%s", username, uniqueSuffix)

	err := userService.RegisterUser(context.Background(), &models.RegisterRequest{
		Username: uniqueUsername,
		Email:    uniqueEmail,
		Password: "password123",
		Role:     role,
	})
	require.NoError(t, err)

	// Ищем пользователя по уникальному email
	user, err := userRepo.GetUserByEmail(context.Background(), uniqueEmail)
	require.NoError(t, err)
	return user
}

// setupTestTask создает задачу для указанного заказчика.
func setupTestTask(t *testing.T, customerID int, budget float64) *models.Task {
	taskRepo := repository.NewTaskRepository(testDB)
	task := &models.Task{
		Title:       "Integration Test Task",
		Description: "For testing business rules",
		CustomerID:  customerID,
		Budget:      budget,
		Status:      models.TaskStatusOpen,
	}
	err := taskRepo.CreateTask(context.Background(), task)
	require.NoError(t, err)
	return task
}

// setupTestBid создает отклик на задачу от указанного исполнителя.
func setupTestBid(t *testing.T, taskID, executorID int, amount float64) *models.Bid {
	bidRepo := repository.NewBidRepository(testDB)
	bid := &models.Bid{
		TaskID:     taskID,
		ExecutorID: executorID,
		Amount:     amount,
		Status:     models.BidStatusPending,
	}
	err := bidRepo.CreateBid(context.Background(), bid)
	require.NoError(t, err)
	return bid
}

// --- НАЧАЛО ТЕСТОВ ---

// Тест 1: Подтверждение выполнения успешно (атомарная транзакция)
func TestTaskService_ConfirmTaskCompletion_Success(t *testing.T) {
	customer := setupTestUser(t, "cust_1", "cust1@test.com", "customer")
	executor := setupTestUser(t, "exec_1", "exec1@test.com", "executor")
	task := setupTestTask(t, customer.ID, 100.0)
	bid := setupTestBid(t, task.ID, executor.ID, 80.0)

	// Устанавливаем баланс заказчику до 100
	_, err := testDB.Exec("UPDATE users SET balance = 100.0 WHERE id = $1", customer.ID)
	require.NoError(t, err)

	// Выбираем отклик и переводим задачу в in_progress
	_, err = testDB.Exec("UPDATE bids SET status = 'selected' WHERE id = $1", bid.ID)
	require.NoError(t, err)
	_, err = testDB.Exec("UPDATE tasks SET status = 'in_progress' WHERE id = $1", task.ID)
	require.NoError(t, err)

	// Помечаем отклик как выполненный через BidService
	bidService := NewBidService(
		repository.NewBidRepository(testDB),
		repository.NewTaskRepository(testDB),
		repository.NewUserRepository(testDB),
		testDB,
	)
	err = bidService.MarkBidCompleted(context.Background(), bid.ID, executor.ID)
	require.NoError(t, err)

	// Создаем TaskService и вызываем подтверждение
	taskService := NewTaskService(
		repository.NewTaskRepository(testDB),
		repository.NewBidRepository(testDB),
		repository.NewUserRepository(testDB),
		repository.NewPaymentRepository(testDB),
		testDB,
	)

	err = taskService.ConfirmTaskCompletion(context.Background(), task.ID, customer.ID)
	assert.NoError(t, err)

	// Проверка результатов в БД
	var taskStatus models.TaskStatus
	err = testDB.QueryRow("SELECT status FROM tasks WHERE id = $1", task.ID).Scan(&taskStatus)
	assert.NoError(t, err)
	assert.Equal(t, models.TaskStatusCompleted, taskStatus)

	var custBalance, execBalance float64
	err = testDB.QueryRow("SELECT balance FROM users WHERE id = $1", customer.ID).Scan(&custBalance)
	assert.NoError(t, err)
	assert.Equal(t, 20.0, custBalance) // 100 - 80

	err = testDB.QueryRow("SELECT balance FROM users WHERE id = $1", executor.ID).Scan(&execBalance)
	assert.NoError(t, err)
	assert.Equal(t, 80.0, execBalance) // 0 + 80

	var paymentCount int
	err = testDB.QueryRow("SELECT COUNT(*) FROM payments WHERE task_id = $1 AND from_user_id = $2 AND to_user_id = $3", task.ID, customer.ID, executor.ID).Scan(&paymentCount)
	assert.NoError(t, err)
	assert.Equal(t, 1, paymentCount)
}

// Тест 2: Подтверждение - недостаточно баллов (транзакция откатывается)
func TestTaskService_ConfirmTaskCompletion_InsufficientBalance(t *testing.T) {
	customer := setupTestUser(t, "cust_2", "cust2@test.com", "customer")
	executor := setupTestUser(t, "exec_2", "exec2@test.com", "executor")
	task := setupTestTask(t, customer.ID, 100.0)
	bid := setupTestBid(t, task.ID, executor.ID, 80.0)

	// Баланс заказчика = 50 < 80
	_, err := testDB.Exec("UPDATE users SET balance = 50.0 WHERE id = $1", customer.ID)
	require.NoError(t, err)

	// Выбираем отклик
	_, err = testDB.Exec("UPDATE bids SET status = 'selected' WHERE id = $1", bid.ID)
	require.NoError(t, err)
	_, err = testDB.Exec("UPDATE tasks SET status = 'in_progress' WHERE id = $1", task.ID)
	require.NoError(t, err)

	bidService := NewBidService(
		repository.NewBidRepository(testDB),
		repository.NewTaskRepository(testDB),
		repository.NewUserRepository(testDB),
		testDB,
	)
	err = bidService.MarkBidCompleted(context.Background(), bid.ID, executor.ID)
	require.NoError(t, err)

	taskService := NewTaskService(
		repository.NewTaskRepository(testDB),
		repository.NewBidRepository(testDB),
		repository.NewUserRepository(testDB),
		repository.NewPaymentRepository(testDB),
		testDB,
	)

	err = taskService.ConfirmTaskCompletion(context.Background(), task.ID, customer.ID)

	// Используем ErrorIs для типизированной ошибки
	assert.ErrorIs(t, err, models.ErrInsufficientBalance)

	// Проверка: балансы НЕ изменились
	var custBal, execBal float64
	err = testDB.QueryRow("SELECT balance FROM users WHERE id = $1", customer.ID).Scan(&custBal)
	assert.NoError(t, err)
	assert.Equal(t, 50.0, custBal)

	err = testDB.QueryRow("SELECT balance FROM users WHERE id = $1", executor.ID).Scan(&execBal)
	assert.NoError(t, err)
	assert.Equal(t, 0.0, execBal)

	// Проверка: статус задачи НЕ изменился на completed
	var taskStatus models.TaskStatus
	err = testDB.QueryRow("SELECT status FROM tasks WHERE id = $1", task.ID).Scan(&taskStatus)
	assert.NoError(t, err)
	assert.Equal(t, models.TaskStatusInProgress, taskStatus)

	// Проверка: платеж НЕ создан
	var paymentCount int
	err = testDB.QueryRow("SELECT COUNT(*) FROM payments WHERE task_id = $1", task.ID).Scan(&paymentCount)
	assert.NoError(t, err)
	assert.Equal(t, 0, paymentCount)
}

// Тест 3: Подтверждение - задача не в in_progress
func TestTaskService_ConfirmTaskCompletion_InvalidStatus(t *testing.T) {
	customer := setupTestUser(t, "cust_3", "cust3@test.com", "customer")
	task := setupTestTask(t, customer.ID, 100.0) // Статус open

	taskService := NewTaskService(
		repository.NewTaskRepository(testDB),
		repository.NewBidRepository(testDB),
		repository.NewUserRepository(testDB),
		repository.NewPaymentRepository(testDB),
		testDB,
	)

	err := taskService.ConfirmTaskCompletion(context.Background(), task.ID, customer.ID)
	assert.Error(t, err)
	// Оставляем Contains, так как ошибка содержит динамический статус
	assert.Contains(t, err.Error(), "must be in_progress")
}

// Тест 4: Подтверждение - не владелец задачи
func TestTaskService_ConfirmTaskCompletion_NotOwner(t *testing.T) {
	customer1 := setupTestUser(t, "cust_4a", "cust4a@test.com", "customer")
	customer2 := setupTestUser(t, "cust_4b", "cust4b@test.com", "customer")
	task := setupTestTask(t, customer1.ID, 100.0)

	// Переводим в in_progress
	_, err := testDB.Exec("UPDATE tasks SET status = 'in_progress' WHERE id = $1", task.ID)
	require.NoError(t, err)

	taskService := NewTaskService(
		repository.NewTaskRepository(testDB),
		repository.NewBidRepository(testDB),
		repository.NewUserRepository(testDB),
		repository.NewPaymentRepository(testDB),
		testDB,
	)

	err = taskService.ConfirmTaskCompletion(context.Background(), task.ID, customer2.ID)
	// Используем ErrorIs для типизированной ошибки
	assert.ErrorIs(t, err, models.ErrOnlyTaskOwner)
}

// Тест 5: Отмена - нельзя отменить выполненную задачу
func TestTaskService_CancelTask_CannotCancelCompleted(t *testing.T) {
	customer := setupTestUser(t, "cust_5", "cust5@test.com", "customer")
	task := setupTestTask(t, customer.ID, 100.0)

	// Переводим задачу в completed
	_, err := testDB.Exec("UPDATE tasks SET status = 'completed' WHERE id = $1", task.ID)
	require.NoError(t, err)

	taskService := NewTaskService(
		repository.NewTaskRepository(testDB),
		repository.NewBidRepository(testDB),
		repository.NewUserRepository(testDB),
		repository.NewPaymentRepository(testDB),
		testDB,
	)

	err = taskService.CancelTask(context.Background(), task.ID, customer.ID)
	assert.Error(t, err)
	// Оставляем Contains, так как это простая строковая ошибка
	assert.Contains(t, err.Error(), "cannot cancel a completed task")

	// Проверка: статус остался completed
	var taskStatus models.TaskStatus
	err = testDB.QueryRow("SELECT status FROM tasks WHERE id = $1", task.ID).Scan(&taskStatus)
	assert.NoError(t, err)
	assert.Equal(t, models.TaskStatusCompleted, taskStatus)
}

// Тест 6: Отмена - успешно для open задачи
func TestTaskService_CancelTask_Open(t *testing.T) {
	customer := setupTestUser(t, "cust_6", "cust6@test.com", "customer")
	task := setupTestTask(t, customer.ID, 100.0) // Статус open

	taskService := NewTaskService(
		repository.NewTaskRepository(testDB),
		repository.NewBidRepository(testDB),
		repository.NewUserRepository(testDB),
		repository.NewPaymentRepository(testDB),
		testDB,
	)

	err := taskService.CancelTask(context.Background(), task.ID, customer.ID)
	assert.NoError(t, err)

	var taskStatus models.TaskStatus
	err = testDB.QueryRow("SELECT status FROM tasks WHERE id = $1", task.ID).Scan(&taskStatus)
	assert.NoError(t, err)
	assert.Equal(t, models.TaskStatusCancelled, taskStatus)
}

// Тест 7: Создание задачи - только customer
func TestTaskService_CreateTask_CustomerOnly(t *testing.T) {
	executor := setupTestUser(t, "exec_7", "exec7@test.com", "executor")
	req := &models.CreateTaskRequest{Title: "Test", Budget: 50.0}

	taskService := NewTaskService(
		repository.NewTaskRepository(testDB),
		repository.NewBidRepository(testDB),
		repository.NewUserRepository(testDB),
		repository.NewPaymentRepository(testDB),
		testDB,
	)

	_, err := taskService.CreateTask(context.Background(), executor.ID, req)
	// Используем ErrorIs для типизированной ошибки
	assert.ErrorIs(t, err, models.ErrOnlyCustomer)
}

// Тест 8: Выбор отклика - не владелец задачи
func TestTaskService_SelectBid_NotOwner(t *testing.T) {
	customer1 := setupTestUser(t, "cust_8a", "cust8a@test.com", "customer")
	customer2 := setupTestUser(t, "cust_8b", "cust8b@test.com", "customer")
	task := setupTestTask(t, customer1.ID, 100.0)
	bid := setupTestBid(t, task.ID, customer1.ID, 50.0)

	taskService := NewTaskService(
		repository.NewTaskRepository(testDB),
		repository.NewBidRepository(testDB),
		repository.NewUserRepository(testDB),
		repository.NewPaymentRepository(testDB),
		testDB,
	)

	err := taskService.SelectBid(context.Background(), task.ID, customer2.ID, bid.ID)
	// Используем ErrorIs для типизированной ошибки
	assert.ErrorIs(t, err, models.ErrOnlyTaskOwner)
}

// Тест 9: Создание отклика - не executor
func TestBidService_CreateBid_NotExecutor(t *testing.T) {
	customer := setupTestUser(t, "cust_9", "cust9@test.com", "customer")
	task := setupTestTask(t, customer.ID, 100.0)

	bidService := NewBidService(
		repository.NewBidRepository(testDB),
		repository.NewTaskRepository(testDB),
		repository.NewUserRepository(testDB),
		testDB,
	)

	req := &models.CreateBidRequest{Amount: 75.0}
	_, err := bidService.CreateBid(context.Background(), customer.ID, task.ID, req)
	// Используем ErrorIs для типизированной ошибки
	assert.ErrorIs(t, err, models.ErrOnlyExecutor)
}

// Тест 10: Выбор отклика - задача не open
func TestTaskService_SelectBid_TaskNotOpen(t *testing.T) {
	customer := setupTestUser(t, "cust_10", "cust10@test.com", "customer")
	task := setupTestTask(t, customer.ID, 100.0)
	bid := setupTestBid(t, task.ID, customer.ID, 50.0)

	// Переводим задачу в completed
	_, err := testDB.Exec("UPDATE tasks SET status = 'completed' WHERE id = $1", task.ID)
	require.NoError(t, err)

	taskService := NewTaskService(
		repository.NewTaskRepository(testDB),
		repository.NewBidRepository(testDB),
		repository.NewUserRepository(testDB),
		repository.NewPaymentRepository(testDB),
		testDB,
	)

	err = taskService.SelectBid(context.Background(), task.ID, customer.ID, bid.ID)
	assert.Error(t, err)
	// Оставляем Contains, так как ошибка содержит динамический статус
	assert.Contains(t, err.Error(), "can only select bid for open tasks")
}

// Тест 11: Создание отклика - уже есть отклик от этого исполнителя
func TestBidService_CreateBid_AlreadyExists(t *testing.T) {
	customer := setupTestUser(t, "cust_11", "cust11@test.com", "customer")
	executor := setupTestUser(t, "exec_11", "exec11@test.com", "executor")
	task := setupTestTask(t, customer.ID, 100.0)

	// Создаём первый отклик
	setupTestBid(t, task.ID, executor.ID, 50.0)

	bidService := NewBidService(
		repository.NewBidRepository(testDB),
		repository.NewTaskRepository(testDB),
		repository.NewUserRepository(testDB),
		testDB,
	)

	req := &models.CreateBidRequest{Amount: 60.0}
	_, err := bidService.CreateBid(context.Background(), executor.ID, task.ID, req)
	// Используем ErrorIs для типизированной ошибки
	assert.ErrorIs(t, err, models.ErrAlreadyExists)

	// Проверка: в БД только один отклик от этого исполнителя
	var count int
	err = testDB.QueryRow("SELECT COUNT(*) FROM bids WHERE task_id = $1 AND executor_id = $2", task.ID, executor.ID).Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}

// Тест 12: Пометить выполненным - не выбранный исполнитель
func TestBidService_MarkBidCompleted_NotSelected(t *testing.T) {
	customer := setupTestUser(t, "cust_12", "cust12@test.com", "customer")
	executor1 := setupTestUser(t, "exec_12a", "exec12a@test.com", "executor")
	executor2 := setupTestUser(t, "exec_12b", "exec12b@test.com", "executor")
	task := setupTestTask(t, customer.ID, 100.0)
	bid1 := setupTestBid(t, task.ID, executor1.ID, 50.0) // Будет selected
	bid2 := setupTestBid(t, task.ID, executor2.ID, 60.0) // Не будет selected

	// Выбираем bid1
	_, err := testDB.Exec("UPDATE bids SET status = 'selected' WHERE id = $1", bid1.ID)
	require.NoError(t, err)
	_, err = testDB.Exec("UPDATE tasks SET status = 'in_progress' WHERE id = $1", task.ID)
	require.NoError(t, err)

	bidService := NewBidService(
		repository.NewBidRepository(testDB),
		repository.NewTaskRepository(testDB),
		repository.NewUserRepository(testDB),
		testDB,
	)

	// executor2 пытается пометить свой bid2 (статус pending) как выполненный
	err = bidService.MarkBidCompleted(context.Background(), bid2.ID, executor2.ID)
	assert.Error(t, err)
	// Оставляем Contains, так как ошибка содержит динамический статус
	assert.Contains(t, err.Error(), "can only complete selected bids")

	// Проверка: статус bid2 остался pending
	var bid2Status models.BidStatus
	err = testDB.QueryRow("SELECT status FROM bids WHERE id = $1", bid2.ID).Scan(&bid2Status)
	assert.NoError(t, err)
	assert.Equal(t, models.BidStatusPending, bid2Status)
}

// Тест 13: Подтверждение выполнения задачи администратором
func TestTaskService_ConfirmTaskCompletion_ByAdmin(t *testing.T) {
	customer := setupTestUser(t, "cust_admin_conf", "cust_admin_conf@example.com", "customer")
	executor := setupTestUser(t, "exec_admin_conf", "exec_admin_conf@example.com", "executor")
	task := setupTestTask(t, customer.ID, 100.0)
	bid := setupTestBid(t, task.ID, executor.ID, 80.0)

	// Устанавливаем баланс заказчику до 100
	_, err := testDB.Exec("UPDATE users SET balance = 100.0 WHERE id = $1", customer.ID)
	require.NoError(t, err)

	// Выбираем отклик и переводим задачу в in_progress
	_, err = testDB.Exec("UPDATE bids SET status = 'selected' WHERE id = $1", bid.ID)
	require.NoError(t, err)
	_, err = testDB.Exec("UPDATE tasks SET status = 'in_progress' WHERE id = $1", task.ID)
	require.NoError(t, err)

	// Помечаем отклик как выполненный через BidService
	bidService := NewBidService(
		repository.NewBidRepository(testDB),
		repository.NewTaskRepository(testDB),
		repository.NewUserRepository(testDB),
		testDB,
	)
	err = bidService.MarkBidCompleted(context.Background(), bid.ID, executor.ID)
	require.NoError(t, err)

	// Создаём администратора
	admin := setupTestUser(t, "admin_conf_test", "admin_conf_test@example.com", "admin")

	// Создаем TaskService и вызываем подтверждение от имени администратора
	taskService := NewTaskService(
		repository.NewTaskRepository(testDB),
		repository.NewBidRepository(testDB),
		repository.NewUserRepository(testDB),
		repository.NewPaymentRepository(testDB),
		testDB,
	)

	// ВАЖНО: Передаём ID администратора (admin.ID), а не заказчика (customer.ID)
	err = taskService.ConfirmTaskCompletion(context.Background(), task.ID, admin.ID)
	assert.NoError(t, err)

	// Проверка результатов в БД
	var taskStatus models.TaskStatus
	err = testDB.QueryRow("SELECT status FROM tasks WHERE id = $1", task.ID).Scan(&taskStatus)
	assert.NoError(t, err)
	assert.Equal(t, models.TaskStatusCompleted, taskStatus)

	var custBalance, execBalance float64
	err = testDB.QueryRow("SELECT balance FROM users WHERE id = $1", customer.ID).Scan(&custBalance)
	assert.NoError(t, err)
	assert.Equal(t, 20.0, custBalance) // 100 - 80

	err = testDB.QueryRow("SELECT balance FROM users WHERE id = $1", executor.ID).Scan(&execBalance)
	assert.NoError(t, err)
	assert.Equal(t, 80.0, execBalance) // 0 + 80

	var paymentCount int
	err = testDB.QueryRow("SELECT COUNT(*) FROM payments WHERE task_id = $1 AND from_user_id = $2 AND to_user_id = $3", task.ID, customer.ID, executor.ID).Scan(&paymentCount)
	assert.NoError(t, err)
	assert.Equal(t, 1, paymentCount)
}
