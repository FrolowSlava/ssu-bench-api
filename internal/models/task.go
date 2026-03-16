package models

import "time"

type TaskStatus string

const (
	TaskStatusOpen       TaskStatus = "open"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

type Task struct {
	ID          int        `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	CustomerID  int        `json:"customer_id"`
	Budget      float64    `json:"budget"`
	Status      TaskStatus `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	// Дополнительно для ответов
	CustomerUsername string `json:"customer_username,omitempty"`
	SelectedBidID    *int   `json:"selected_bid_id,omitempty"`
}

type CreateTaskRequest struct {
	Title       string  `json:"title" binding:"required,min=3,max=255"`
	Description string  `json:"description" binding:"max=1000"`
	Budget      float64 `json:"budget" binding:"required,min=0.01"`
}

type UpdateTaskStatusRequest struct {
	Status TaskStatus `json:"status" binding:"oneof=in_progress completed cancelled"`
}

type ListTasksQuery struct {
	Status   string `form:"status"`
	Customer int    `form:"customer"`
	Page     int    `form:"page,default=1"`
	Limit    int    `form:"limit,default=20"`
}
