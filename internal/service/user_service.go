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
	if err == nil {
		return errors.New("user with this email already exists")
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
	}
	return s.userRepo.CreateUser(ctx, user)
}

func (s *UserService) AuthenticateUser(ctx context.Context, email, password string) (*models.User, error) {
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, errors.New("invalid credentials")
	}
	return user, nil
}

func (s *UserService) GetUserByID(ctx context.Context, id int) (*models.User, error) {
	return s.userRepo.GetUserByID(ctx, id)
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
		return err
	}
	if user.Role == models.RoleAdmin {
		return errors.New("cannot block admin user")
	}
	return s.userRepo.UpdateUserBlocked(ctx, id, true)
}

// UnblockUser разблокирует пользователя
func (s *UserService) UnblockUser(ctx context.Context, id int) error {
	return s.userRepo.UpdateUserBlocked(ctx, id, false)
}

// GetUserWithBalance возвращает пользователя с балансом
func (s *UserService) GetUserWithBalance(ctx context.Context, id int) (*models.User, error) {
	return s.userRepo.GetUserWithBalance(ctx, id)
}
