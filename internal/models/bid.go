package models

import "time"

type BidStatus string

const (
	BidStatusPending   BidStatus = "pending"
	BidStatusSelected  BidStatus = "selected"
	BidStatusRejected  BidStatus = "rejected"
	BidStatusCompleted BidStatus = "completed"
)

type Bid struct {
	ID         int       `json:"id"`
	TaskID     int       `json:"task_id"`
	ExecutorID int       `json:"executor_id"`
	Amount     float64   `json:"amount"`
	Status     BidStatus `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Дополнительно для ответов
	ExecutorUsername string `json:"executor_username,omitempty"`
	TaskTitle        string `json:"task_title,omitempty"`
}

type CreateBidRequest struct {
	Amount float64 `json:"amount" binding:"required,min=0.01"`
}

type SelectBidRequest struct {
	BidID int `json:"bid_id" binding:"required,min=1"`
}
