package repository

import (
	"context"
	"database/sql"

	"app/internal/domains/reaction"
	"app/internal/models"

	"gorm.io/gorm"
)

type reactionRepository struct {
	db    *gorm.DB
	sqlDB *sql.DB
}

func NewReactionRepository(db *gorm.DB) reaction.ReactionRepository {
	sqlDB, err := db.DB()
	if err != nil {
		panic(err)
	}
	return &reactionRepository{db: db, sqlDB: sqlDB}
}

func (r *reactionRepository) List(ctx context.Context) ([]*models.Reaction, error) {
	rows, err := r.sqlDB.QueryContext(ctx, "SELECT slug, COALESCE(love, 0) FROM reactions ORDER BY slug ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reactions []*models.Reaction
	for rows.Next() {
		var rec models.Reaction
		if err := rows.Scan(&rec.Slug, &rec.Love); err != nil {
			return nil, err
		}
		reactions = append(reactions, &rec)
	}
	return reactions, rows.Err()
}

func (r *reactionRepository) GetBySlug(ctx context.Context, slug string) (*models.Reaction, error) {
	var rec models.Reaction
	err := r.sqlDB.QueryRowContext(ctx, "SELECT slug, COALESCE(love, 0) FROM reactions WHERE slug = ?", slug).Scan(&rec.Slug, &rec.Love)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *reactionRepository) Add(ctx context.Context, slug string) error {
	var existing int64
	r.sqlDB.QueryRowContext(ctx, "SELECT love FROM reactions WHERE slug = ?", slug).Scan(&existing)
	if existing == 0 {
		_, err := r.sqlDB.ExecContext(ctx, "INSERT INTO reactions (slug, love) VALUES (?, 1)", slug)
		return err
	}
	_, err := r.sqlDB.ExecContext(ctx, "UPDATE reactions SET love = love + 1 WHERE slug = ?", slug)
	return err
}

func (r *reactionRepository) Remove(ctx context.Context, slug string) error {
	_, err := r.sqlDB.ExecContext(ctx, "UPDATE reactions SET love = MAX(love - 1, 0) WHERE slug = ?", slug)
	return err
}
