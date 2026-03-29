package handler

import (
	"errors"
	"net/http"
	"ssu-bench-api/internal/models"
	"ssu-bench-api/internal/service"
	"strconv"

	"github.com/gin-gonic/gin"
)

type TaskHandler struct {
	taskService    *service.TaskService
	bidService     *service.BidService
	paymentService *service.PaymentService
}

func NewTaskHandler(taskService *service.TaskService, bidService *service.BidService, paymentService *service.PaymentService) *TaskHandler {
	return &TaskHandler{
		taskService:    taskService,
		bidService:     bidService,
		paymentService: paymentService,
	}
}

func getRoleFromContext(c *gin.Context) (string, error) {
	roleRaw, exists := c.Get("role")
	if !exists {
		return "", nil
	}
	switch v := roleRaw.(type) {
	case string:
		return v, nil
	case models.Role:
		return string(v), nil
	default:
		return "", nil
	}
}

func getUserIDFromContext(c *gin.Context) (int, error) {
	userIDRaw, exists := c.Get("user_id")
	if !exists {
		return 0, nil
	}
	switch v := userIDRaw.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case int64:
		return int(v), nil
	default:
		return 0, nil
	}
}

// CreateTask создаёт новую задачу (только customer/admin)
// POST /api/v1/tasks
func (h *TaskHandler) CreateTask(c *gin.Context) {
	userRole, err := getRoleFromContext(c)
	if err != nil || (userRole != "customer" && userRole != "admin") {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "forbidden",
			Details: "only customers can create tasks",
		})
		return
	}
	userID, err := getUserIDFromContext(c)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error:   "unauthorized",
			Details: "invalid user_id",
		})
		return
	}
	var req models.CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Details: err.Error(),
		})
		return
	}
	ctx := c.Request.Context()
	task, err := h.taskService.CreateTask(ctx, userID, &req)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Details: "customer not found",
			})
			return
		}
		if errors.Is(err, models.ErrOnlyCustomer) {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "forbidden",
				Details: "only customers can create tasks",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "internal_error",
			Details: err.Error(),
		})
		return
	}
	c.JSON(http.StatusCreated, task)
}

// GetTasks возвращает список задач с пагинацией и фильтрами
// GET /api/v1/tasks?page=1&limit=20&status=open&customer=123
func (h *TaskHandler) GetTasks(c *gin.Context) {
	var query models.ListTasksQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Details: err.Error(),
		})
		return
	}
	// Устанавливаем значения по умолчанию (Gin не делает это автоматически для form-тегов)
	if query.Page < 1 {
		query.Page = 1
	}
	if query.Limit < 1 || query.Limit > 100 {
		query.Limit = 20
	}
	ctx := c.Request.Context()
	tasks, total, err := h.taskService.ListTasks(ctx, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "internal_error",
			Details: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
		"pagination": gin.H{
			"page":  query.Page,
			"limit": query.Limit,
			"total": total,
		},
	})
}

// GetTask возвращает задачу по ID
// GET /api/v1/tasks/:id
func (h *TaskHandler) GetTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Details: "invalid task id",
		})
		return
	}
	ctx := c.Request.Context()
	task, err := h.taskService.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrTaskNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Details: "task not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "internal_error",
			Details: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, task)
}

// CreateBid создаёт отклик на задачу (только executor/admin)
// POST /api/v1/tasks/:id/bids
func (h *TaskHandler) CreateBid(c *gin.Context) {
	userRole, err := getRoleFromContext(c)
	if err != nil || (userRole != "executor" && userRole != "admin") {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "forbidden",
			Details: "only executors can create bids",
		})
		return
	}
	userID, err := getUserIDFromContext(c)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error:   "unauthorized",
			Details: "invalid user_id",
		})
		return
	}
	taskID, err := strconv.Atoi(c.Param("id"))
	if err != nil || taskID <= 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Details: "invalid task id",
		})
		return
	}
	var req models.CreateBidRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Details: err.Error(),
		})
		return
	}
	ctx := c.Request.Context()
	bid, err := h.bidService.CreateBid(ctx, userID, taskID, &req)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) || errors.Is(err, models.ErrTaskNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Details: err.Error(),
			})
			return
		}
		if errors.Is(err, models.ErrOnlyExecutor) {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "forbidden",
				Details: "only executors can create bids",
			})
			return
		}
		if errors.Is(err, models.ErrAlreadyExists) {
			c.JSON(http.StatusConflict, models.ErrorResponse{
				Error:   "conflict",
				Details: "you have already bid on this task",
			})
			return
		}
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Error:   "conflict",
			Details: err.Error(),
		})
		return
	}
	c.JSON(http.StatusCreated, bid)
}

// SelectBid выбирает исполнителя для задачи (только customer-владелец)
// Бизнес-правила:
// - Только владелец задачи может выбрать исполнителя
// - Только для задач со статусом "open"
// - Атомарно: одна заявка selected, остальные rejected
// POST /api/v1/tasks/:id/select-bid
func (h *TaskHandler) SelectBid(c *gin.Context) {
	userRole, err := getRoleFromContext(c)
	if err != nil || (userRole != "customer" && userRole != "admin") {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "forbidden",
			Details: "only task owner can select bid",
		})
		return
	}
	userID, err := getUserIDFromContext(c)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error:   "unauthorized",
			Details: "invalid user_id",
		})
		return
	}
	taskID, err := strconv.Atoi(c.Param("id"))
	if err != nil || taskID <= 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Details: "invalid task id",
		})
		return
	}
	var req models.SelectBidRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Details: err.Error(),
		})
		return
	}
	ctx := c.Request.Context()
	if err := h.taskService.SelectBid(ctx, taskID, userID, req.BidID); err != nil {
		if errors.Is(err, models.ErrTaskNotFound) || errors.Is(err, models.ErrBidNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Details: err.Error(),
			})
			return
		}
		if errors.Is(err, models.ErrOnlyTaskOwner) {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "forbidden",
				Details: "only task owner can select bid",
			})
			return
		}
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Error:   "conflict",
			Details: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "bid selected", "bid_id": req.BidID})
}

// ConfirmCompletion подтверждает выполнение задачи и переводит баллы
// Атомарная операция в транзакции:
// 1. Проверка прав заказчика
// 2. Проверка статуса задачи и отклика
// 3. Проверка баланса заказчика
// 4. Списание баллов у заказчика
// 5. Начисление баллов исполнителю
// 6. Запись платежа в историю
// 7. Обновление статуса задачи на "completed"
// POST /api/v1/tasks/:id/confirm
func (h *TaskHandler) ConfirmCompletion(c *gin.Context) {
	userRole, err := getRoleFromContext(c)
	if err != nil || (userRole != "customer" && userRole != "admin") {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "forbidden",
			Details: "only task owner can confirm completion",
		})
		return
	}
	userID, err := getUserIDFromContext(c)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error:   "unauthorized",
			Details: "invalid user_id",
		})
		return
	}
	taskID, err := strconv.Atoi(c.Param("id"))
	if err != nil || taskID <= 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Details: "invalid task id",
		})
		return
	}
	ctx := c.Request.Context()
	if err := h.taskService.ConfirmTaskCompletion(ctx, taskID, userID); err != nil {
		switch {
		case errors.Is(err, models.ErrInsufficientBalance):
			c.JSON(http.StatusPaymentRequired, models.ErrorResponse{
				Error:   "insufficient_balance",
				Details: "insufficient balance to complete payment",
			})
		case errors.Is(err, models.ErrTaskNotFound):
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Details: "task not found",
			})
		case errors.Is(err, models.ErrOnlyTaskOwner):
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "forbidden",
				Details: "only task owner can confirm completion",
			})
		default:
			c.JSON(http.StatusConflict, models.ErrorResponse{
				Error:   "conflict",
				Details: err.Error(),
			})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "completion confirmed, payment processed"})
}

// CancelTask отменяет задачу (только владелец, если не выполнена)
// Бизнес-правила:
// - Нельзя отменить выполненную задачу (статус "completed")
// - Можно отменить только "open" или "in_progress"
// POST /api/v1/tasks/:id/cancel
func (h *TaskHandler) CancelTask(c *gin.Context) {
	userRole, err := getRoleFromContext(c)
	if err != nil || (userRole != "customer" && userRole != "admin") {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "forbidden",
			Details: "only task owner can cancel",
		})
		return
	}
	userID, err := getUserIDFromContext(c)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error:   "unauthorized",
			Details: "invalid user_id",
		})
		return
	}
	taskID, err := strconv.Atoi(c.Param("id"))
	if err != nil || taskID <= 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Details: "invalid task id",
		})
		return
	}
	ctx := c.Request.Context()
	if err := h.taskService.CancelTask(ctx, taskID, userID); err != nil {
		if errors.Is(err, models.ErrTaskNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Details: "task not found",
			})
			return
		}
		if errors.Is(err, models.ErrOnlyTaskOwner) {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "forbidden",
				Details: "only task owner can cancel",
			})
			return
		}
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Error:   "conflict",
			Details: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "task cancelled", "task_id": taskID})
}

// MarkBidCompleted помечает заявку как выполненную (только выбранный исполнитель)
// Бизнес-правила:
// - Только исполнитель этой заявки может её завершить
// - Только заявки со статусом "selected" можно завершить
// - Нельзя завершить уже выполненную задачу
// POST /api/v1/bids/:id/complete
func (h *TaskHandler) MarkBidCompleted(c *gin.Context) {
	userRole, err := getRoleFromContext(c)
	if err != nil || (userRole != "executor" && userRole != "admin") {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "forbidden",
			Details: "only executor can mark as completed",
		})
		return
	}
	userID, err := getUserIDFromContext(c)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error:   "unauthorized",
			Details: "invalid user_id",
		})
		return
	}
	bidID, err := strconv.Atoi(c.Param("id"))
	if err != nil || bidID <= 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Details: "invalid bid id",
		})
		return
	}
	ctx := c.Request.Context()
	if err := h.bidService.MarkBidCompleted(ctx, bidID, userID); err != nil {
		if errors.Is(err, models.ErrBidNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Details: "bid not found",
			})
			return
		}
		if errors.Is(err, models.ErrOnlyExecutor) {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "forbidden",
				Details: "only executor can mark as completed",
			})
			return
		}
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Error:   "conflict",
			Details: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "bid marked as completed", "bid_id": bidID})
}
