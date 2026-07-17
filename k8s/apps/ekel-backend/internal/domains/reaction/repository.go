package reaction

import (
	"app/internal/models"
	"context"
)

type ReactionRepository interface {
	List(ctx context.Context) ([]*models.Reaction, error)
	GetBySlug(ctx context.Context, slug string) (*models.Reaction, error)
	Add(ctx context.Context, slug string) error
	Remove(ctx context.Context, slug string) error
}
