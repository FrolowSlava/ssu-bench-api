package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"ssu-bench-api/internal/models"
	"ssu-bench-api/internal/repository"
)

type BidService struct {
	bidRepo  *repository.BidRepository
	taskRepo *repository.TaskRepository
	userRepo *repository.UserRepository
	db       *sql.DB
}

func NewBidService(bidRepo *repository.BidRepository, taskRepo *repository.TaskRepository, userRepo *repository.UserRepository, db *sql.DB) *BidService {
	return &BidService{
		bidRepo:  bidRepo,
		taskRepo: taskRepo,
		userRepo: userRepo,
		db:       db,
	}
}

func (s *BidService) CreateBid(ctx context.Context, executorID, taskID int, req *models.CreateBidRequest) (*models.Bid, error) {
	executor, err := s.userRepo.GetUserByID(ctx, executorID)
	if err != nil {
		return nil, fmt.Errorf("executor not found: %w", err)
	}
	if executor.Role != models.RoleExecutor && executor.Role != models.RoleAdmin {
		return nil, errors.New("only executors can create bids")
	}
	task, err := s.taskRepo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}
	if task.Status != models.TaskStatusOpen {
		return nil, fmt.Errorf("can only bid on open tasks, current status: %s", task.Status)
	}
	alreadyBid, err := s.bidRepo.ExecutorHasBid(ctx, taskID, executorID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing bid: %w", err)
	}
	if alreadyBid {
		return nil, errors.New("you have already bid on this task")
	}
	bid := &models.Bid{
		TaskID:     taskID,
		ExecutorID: executorID,
		Amount:     req.Amount,
		Status:     models.BidStatusPending,
	}
	if err := s.bidRepo.CreateBid(ctx, bid); err != nil {
		return nil, fmt.Errorf("failed to create bid: %w", err)
	}
	bid.ExecutorUsername = executor.Username
	bid.TaskTitle = task.Title
	return bid, nil
}

func (s *BidService) GetBidsForTask(ctx context.Context, taskID, viewerID int, viewerRole models.Role) ([]models.Bid, error) {
	_, err := s.taskRepo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	bids, err := s.bidRepo.GetBidsByTaskID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if viewerRole == models.RoleAdmin {
		return s.enrichBids(ctx, bids)
	}
	task, _ := s.taskRepo.GetTaskByID(ctx, taskID)
	if task != nil && task.CustomerID == viewerID {
		return s.enrichBids(ctx, bids)
	}
	filtered := make([]models.Bid, 0)
	for _, bid := range bids {
		if bid.ExecutorID == viewerID {
			enriched, _ := s.enrichBid(ctx, bid)
			filtered = append(filtered, enriched)
		}
	}
	return filtered, nil
}

func (s *BidService) enrichBids(ctx context.Context, bids []models.Bid) ([]models.Bid, error) {
	result := make([]models.Bid, len(bids))
	for i, bid := range bids {
		enriched, err := s.enrichBid(ctx, bid)
		if err != nil {
			return nil, err
		}
		result[i] = enriched
	}
	return result, nil
}

func (s *BidService) enrichBid(ctx context.Context, bid models.Bid) (models.Bid, error) {
	executor, err := s.userRepo.GetUserByID(ctx, bid.ExecutorID)
	if err == nil {
		bid.ExecutorUsername = executor.Username
	}
	task, err := s.taskRepo.GetTaskByID(ctx, bid.TaskID)
	if err == nil {
		bid.TaskTitle = task.Title
	}
	return bid, nil
}

func (s *BidService) MarkBidCompleted(ctx context.Context, bidID, executorID int) error {
	bid, err := s.bidRepo.GetBidByID(ctx, bidID)
	if err != nil {
		return fmt.Errorf("bid not found: %w", err)
	}
	if bid.ExecutorID != executorID {
		return errors.New("only selected executor can mark task as completed")
	}
	if bid.Status != models.BidStatusSelected {
		return fmt.Errorf("can only complete selected bids, current status: %s", bid.Status)
	}
	task, err := s.taskRepo.GetTaskByID(ctx, bid.TaskID)
	if err != nil {
		return err
	}
	if task.Status == models.TaskStatusCompleted {
		return errors.New("task already completed")
	}
	if err := s.bidRepo.UpdateBidStatus(ctx, bidID, models.BidStatusCompleted); err != nil {
		return fmt.Errorf("failed to update bid status: %w", err)
	}
	return nil
}

func (s *BidService) ExecutorHasBid(ctx context.Context, taskID, executorID int) (bool, error) {
	return s.bidRepo.ExecutorHasBid(ctx, taskID, executorID)
}
