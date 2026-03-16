package repository

import (
	"context"
	"database/sql"
	"fmt"
	"ssu-bench-api/internal/models"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) CreateUser(ctx context.Context, user *models.User) error {
	query := `
	INSERT INTO users (username, email, password_hash, role, created_at)
	VALUES ($1, $2, $3, $4, NOW())`
	_, err := r.db.ExecContext(ctx, query, user.Username, user.Email, user.PasswordHash, user.Role)
	if err != nil {
		return fmt.Errorf("failed to insert user: %w", err)
	}
	return nil
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `SELECT id, username, email, password_hash, role FROM users WHERE email = $1`
	row := r.db.QueryRowContext(ctx, query, email)
	var user models.User
	err := row.Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}
	return &user, nil
}

func (r *UserRepository) GetUserByID(ctx context.Context, id int) (*models.User, error) {
	query := `SELECT id, username, email, role, balance, created_at, updated_at, blocked
	FROM users WHERE id = $1`
	var user models.User
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Username, &user.Email, &user.Role,
		&user.Balance, &user.CreatedAt, &user.UpdatedAt, &user.Blocked,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// === НОВЫЕ МЕТОДЫ ДЛЯ АДМИНА ===

// GetUsers возвращает список пользователей с пагинацией
func (r *UserRepository) GetUsers(ctx context.Context, page, limit int) ([]models.User, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	query := `SELECT id, username, email, role, balance, created_at, updated_at, blocked
		FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Role,
			&u.Balance, &u.CreatedAt, &u.UpdatedAt, &u.Blocked); err != nil {
			return nil, 0, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, u)
	}

	var total int
	countQuery := `SELECT COUNT(*) FROM users`
	_ = r.db.QueryRowContext(ctx, countQuery).Scan(&total)

	return users, total, nil
}

// UpdateUserBlocked блокирует/разблокирует пользователя
func (r *UserRepository) UpdateUserBlocked(ctx context.Context, id int, blocked bool) error {
	query := `UPDATE users SET blocked = $1, updated_at = NOW() WHERE id = $2`
	result, err := r.db.ExecContext(ctx, query, blocked, id)
	if err != nil {
		return fmt.Errorf("failed to update user blocked status: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// GetUserWithBalance возвращает пользователя с балансом (для админа)
func (r *UserRepository) GetUserWithBalance(ctx context.Context, id int) (*models.User, error) {
	query := `SELECT id, username, email, role, balance, created_at, updated_at, blocked
		FROM users WHERE id = $1`
	var user models.User
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Username, &user.Email, &user.Role,
		&user.Balance, &user.CreatedAt, &user.UpdatedAt, &user.Blocked,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}
