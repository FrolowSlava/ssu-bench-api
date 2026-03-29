package handler

import (
	"errors"
	"net/http"
	"strconv"

	"ssu-bench-api/internal/models"
	"ssu-bench-api/internal/service"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	userService    *service.UserService
	taskService    *service.TaskService
	paymentService *service.PaymentService
}

func NewAdminHandler(
	userService *service.UserService,
	taskService *service.TaskService,
	paymentService *service.PaymentService,
) *AdminHandler {
	return &AdminHandler{
		userService:    userService,
		taskService:    taskService,
		paymentService: paymentService,
	}
}

// ListUsers GET /api/v1/admin/users?page=1&limit=20
func (h *AdminHandler) ListUsers(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	ctx := c.Request.Context()
	users, total, err := h.userService.ListUsers(ctx, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "internal_error",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": total,
		},
	})
}

// GetUserDetails GET /api/v1/admin/users/:id
func (h *AdminHandler) GetUserDetails(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Details: "invalid user id",
		})
		return
	}

	ctx := c.Request.Context()
	user, err := h.userService.GetUserWithBalance(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Details: "user not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "internal_error",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":    user.ID,
		"username":   user.Username,
		"email":      user.Email,
		"role":       user.Role,
		"balance":    user.Balance,
		"blocked":    user.Blocked,
		"created_at": user.CreatedAt,
	})
}

// BlockUser POST /api/v1/admin/users/:id/block
func (h *AdminHandler) BlockUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Details: "invalid user id",
		})
		return
	}

	ctx := c.Request.Context()
	if err := h.userService.BlockUser(ctx, id); err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Details: "user not found",
			})
			return
		}
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Error:   "conflict",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "user blocked",
		"user_id": id,
	})
}

// UnblockUser POST /api/v1/admin/users/:id/unblock
func (h *AdminHandler) UnblockUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Details: "invalid user id",
		})
		return
	}

	ctx := c.Request.Context()
	if err := h.userService.UnblockUser(ctx, id); err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Details: "user not found",
			})
			return
		}
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Error:   "conflict",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "user unblocked",
		"user_id": id,
	})
}

// ListPayments GET /api/v1/admin/payments?page=1&limit=20
func (h *AdminHandler) ListPayments(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	ctx := c.Request.Context()
	payments, total, err := h.paymentService.GetAllPayments(ctx, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "internal_error",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"payments": payments,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": total,
		},
	})
}

// ListAllTasks GET /api/v1/admin/tasks?page=1&limit=20&status=open
func (h *AdminHandler) ListAllTasks(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")
	statusFilter := c.Query("status")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	ctx := c.Request.Context()
	tasks, total, err := h.taskService.GetAllTasks(ctx, page, limit, statusFilter)
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
			"page":  page,
			"limit": limit,
			"total": total,
		},
	})
}
