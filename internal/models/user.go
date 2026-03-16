package models

import "time"

type Role string

const (
	RoleCustomer Role = "customer"
	RoleExecutor Role = "executor"
	RoleAdmin    Role = "admin"
)

type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	Password     string    `json:"-"` // пароль в открытом виде (не сохраняем в базу)
	PasswordHash string    `json:"-"` // хэш пароля (для базы)
	Role         Role      `json:"role"`
	Balance      float64   `json:"balance"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Blocked      bool      `json:"blocked"`
}

// --- DTOs для API ---

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Role     string `json:"role" binding:"oneof=customer executor admin"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginResponse struct {
	Token   string `json:"token"`
	UserID  int    `json:"user_id"`
	Role    string `json:"role"`
	Message string `json:"message"`
}
