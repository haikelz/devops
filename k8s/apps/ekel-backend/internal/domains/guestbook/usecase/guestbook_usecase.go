package usecase

import (
	"context"
	"fmt"
	"time"

	"app/internal/domains/guestbook"
	"app/internal/models"

	"github.com/sirupsen/logrus"
)

type guestbookUsecase struct {
	repository guestbook.GuestbookRepository
}

func NewGuestbookUsecase(repo guestbook.GuestbookRepository) guestbook.GuestbookUsecase {
	return &guestbookUsecase{repository: repo}
}

func (uc *guestbookUsecase) Create(ctx context.Context, req guestbook.CreateGuestbookRequest) (*guestbook.GuestbookResponse, error) {
	entry := &models.GuestbookEntry{
		Username:  req.Username,
		Message:   req.Message,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	if err := uc.repository.Create(ctx, entry); err != nil {
		logrus.WithError(err).Error("failed to create guestbook entry")
		return nil, fmt.Errorf("failed to create guestbook entry: %w", err)
	}

	return uc.toResponse(entry), nil
}

func (uc *guestbookUsecase) GetByID(ctx context.Context, id string) (*guestbook.GuestbookResponse, error) {
	entry, err := uc.repository.GetByID(ctx, id)
	if err != nil {
		logrus.WithError(err).Error("failed to get guestbook entry")
		return nil, fmt.Errorf("failed to get guestbook entry: %w", err)
	}

	return uc.toResponse(entry), nil
}

func (uc *guestbookUsecase) List(ctx context.Context) ([]*guestbook.GuestbookResponse, error) {
	entries, err := uc.repository.List(ctx)
	if err != nil {
		logrus.WithError(err).Error("failed to list guestbook entries")
		return nil, fmt.Errorf("failed to list guestbook entries: %w", err)
	}

	responses := make([]*guestbook.GuestbookResponse, 0, len(entries))
	for _, entry := range entries {
		responses = append(responses, uc.toResponse(entry))
	}

	return responses, nil
}

func (uc *guestbookUsecase) Update(ctx context.Context, id string, req guestbook.UpdateGuestbookRequest) (*guestbook.GuestbookResponse, error) {
	entry, err := uc.repository.GetByID(ctx, id)
	if err != nil {
		logrus.WithError(err).Error("failed to get guestbook entry for update")
		return nil, fmt.Errorf("failed to get guestbook entry: %w", err)
	}

	entry.Username = req.Username
	entry.Message = req.Message

	if err := uc.repository.Update(ctx, entry); err != nil {
		logrus.WithError(err).Error("failed to update guestbook entry")
		return nil, fmt.Errorf("failed to update guestbook entry: %w", err)
	}

	return uc.toResponse(entry), nil
}

func (uc *guestbookUsecase) Delete(ctx context.Context, id string) error {
	if err := uc.repository.Delete(ctx, id); err != nil {
		logrus.WithError(err).Error("failed to delete guestbook entry")
		return fmt.Errorf("failed to delete guestbook entry: %w", err)
	}
	return nil
}

func (uc *guestbookUsecase) toResponse(entry *models.GuestbookEntry) *guestbook.GuestbookResponse {
	return &guestbook.GuestbookResponse{
		ID:        entry.ID,
		Username:  entry.Username,
		Message:   entry.Message,
		CreatedAt: entry.CreatedAt,
	}
}
