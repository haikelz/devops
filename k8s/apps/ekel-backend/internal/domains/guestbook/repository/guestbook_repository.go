package repository

import (
	"context"

	"app/internal/domains/guestbook"
	"app/internal/models"

	"gorm.io/gorm"
)

type guestbookRepository struct {
	db *gorm.DB
}

func NewGuestbookRepository(db *gorm.DB) guestbook.GuestbookRepository {
	return &guestbookRepository{db: db}
}

func (r *guestbookRepository) Create(ctx context.Context, entry *models.GuestbookEntry) error {
	return r.db.WithContext(ctx).Create(entry).Error
}

func (r *guestbookRepository) GetByID(ctx context.Context, id string) (*models.GuestbookEntry, error) {
	var entry models.GuestbookEntry
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&entry).Error
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func (r *guestbookRepository) List(ctx context.Context) ([]*models.GuestbookEntry, error) {
	var entries []*models.GuestbookEntry

	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Find(&entries).Error
	if err != nil {
		return nil, err
	}

	return entries, nil
}

func (r *guestbookRepository) Update(ctx context.Context, entry *models.GuestbookEntry) error {
	return r.db.WithContext(ctx).Save(entry).Error
}

func (r *guestbookRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.GuestbookEntry{}, "id = ?", id).Error
}
