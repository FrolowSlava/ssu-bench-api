package models

import (
	"fmt"
	"time"
)

// PaymentType определяет тип платежа
type PaymentType string

const (
	PaymentTypeReward PaymentType = "reward" // Вознаграждение исполнителю
	PaymentTypeRefund PaymentType = "refund" // Возврат заказчику
)

// Payment — сущность платежа (фиксация перевода баллов)
type Payment struct {
	ID          int         `json:"id"`
	TaskID      int         `json:"task_id"`
	FromUserID  int         `json:"from_user_id"` // Заказчик (списывание)
	ToUserID    int         `json:"to_user_id"`   // Исполнитель (начисление)
	Amount      float64     `json:"amount"`       // Сумма перевода
	Type        PaymentType `json:"type"`         // Тип платежа
	Status      string      `json:"status"`       // pending, completed, failed
	Description string      `json:"description,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
}

// PaymentResult — результат обработки платежа
type PaymentResult struct {
	PaymentID   int     `json:"payment_id"`       // ID записи о платеже (0 если не создан)
	FromBalance float64 `json:"from_balance"`     // Баланс отправителя после операции
	ToBalance   float64 `json:"to_balance"`       // Баланс получателя после операции
	Success     bool    `json:"success"`          // Успешность операции
	Amount      float64 `json:"amount,omitempty"` // Сумма перевода (если успешно)
	Error       string  `json:"error,omitempty"`  // Сообщение об ошибке (если неуспешно)
}

// IsValid проверяет валидность платежа перед обработкой
func (p *Payment) IsValid() error {
	if p.TaskID <= 0 {
		return fmt.Errorf("invalid task_id")
	}
	if p.FromUserID <= 0 || p.ToUserID <= 0 {
		return fmt.Errorf("invalid user ids")
	}
	if p.Amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	if p.Type != PaymentTypeReward && p.Type != PaymentTypeRefund {
		return fmt.Errorf("invalid payment type")
	}
	return nil
}

// IsCompleted возвращает true, если платеж завершён
func (p *Payment) IsCompleted() bool {
	return p.Status == "completed" && p.CompletedAt != nil
}
