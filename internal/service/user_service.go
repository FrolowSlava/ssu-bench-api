package service

import (
	"context"
	"errors"
	"fmt"
	"ssu-bench-api/internal/models"
	"ssu-bench-api/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	userRepo *repository.UserRepository
}

func NewUserService(userRepo *repository.UserRepository) *UserService {
	return &UserService{userRepo: userRepo}
}

func (s *UserService) RegisterUser(ctx context.Context, req *models.RegisterRequest) error {
	_, err := s.userRepo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if !errors.Is(err, models.ErrUserNotFound) {
			return fmt.Errorf("failed to check user existence: %w", err)
		}
	} else {
		return models.ErrAlreadyExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user := &models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		Role:         models.Role(req.Role),
		Balance:      req.Balance,
	}

	if err := s.userRepo.CreateUser(ctx, user); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func (s *UserService) AuthenticateUser(ctx context.Context, email, password string) (*models.User, error) {
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, models.ErrUnauthorized
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, models.ErrUnauthorized
	}

	return user, nil
}

func (s *UserService) GetUserByID(ctx context.Context, id int) (*models.User, error) {
	user, err := s.userRepo.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, models.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

// === НОВЫЕ МЕТОДЫ ДЛЯ АДМИНА ===

// ListUsers возвращает список пользователей с пагинацией
func (s *UserService) ListUsers(ctx context.Context, page, limit int) ([]models.User, int, error) {
	return s.userRepo.GetUsers(ctx, page, limit)
}

// BlockUser блокирует пользователя
func (s *UserService) BlockUser(ctx context.Context, id int) error {
	user, err := s.userRepo.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return models.ErrUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user.Role == models.RoleAdmin {
		return models.ErrCannotBlockAdmin
	}
	if err := s.userRepo.UpdateUserBlocked(ctx, id, true); err != nil {
		return fmt.Errorf("failed to block user: %w", err)
	}
	return nil
}

// UnblockUser разблокирует пользователя
func (s *UserService) UnblockUser(ctx context.Context, id int) error {
	_, err := s.userRepo.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return models.ErrUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}
	if err := s.userRepo.UpdateUserBlocked(ctx, id, false); err != nil {
		return fmt.Errorf("failed to unblock user: %w", err)
	}
	return nil
}

// GetUserWithBalance возвращает пользователя с балансом
func (s *UserService) GetUserWithBalance(ctx context.Context, id int) (*models.User, error) {
	user, err := s.userRepo.GetUserWithBalance(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, models.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}
