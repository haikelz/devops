package usecase

import (
	"context"
	"fmt"

	"app/internal/domains/reaction"
	"app/internal/models"

	"github.com/sirupsen/logrus"
)

type reactionUsecase struct {
	repository reaction.ReactionRepository
}

func NewReactionUsecase(repo reaction.ReactionRepository) reaction.ReactionUsecase {
	return &reactionUsecase{repository: repo}
}

func (uc *reactionUsecase) List(ctx context.Context) ([]*reaction.ReactionResponse, error) {
	reactions, err := uc.repository.List(ctx)
	if err != nil {
		logrus.WithError(err).Error("failed to list reactions")
		return nil, fmt.Errorf("failed to list reactions: %w", err)
	}

	responses := make([]*reaction.ReactionResponse, 0, len(reactions))
	for _, r := range reactions {
		responses = append(responses, uc.toResponse(r))
	}

	return responses, nil
}

func (uc *reactionUsecase) Add(ctx context.Context, slug string) (*reaction.ReactionResponse, error) {
	if err := uc.repository.Add(ctx, slug); err != nil {
		logrus.WithError(err).Error("failed to add reaction")
		return nil, fmt.Errorf("failed to add reaction: %w", err)
	}

	r, _ := uc.repository.GetBySlug(ctx, slug)
	if r != nil {
		return uc.toResponse(r), nil
	}
	return &reaction.ReactionResponse{Slug: slug, Love: 1}, nil
}

func (uc *reactionUsecase) Remove(ctx context.Context, slug string) (*reaction.ReactionResponse, error) {
	if err := uc.repository.Remove(ctx, slug); err != nil {
		logrus.WithError(err).Error("failed to remove reaction")
		return nil, fmt.Errorf("failed to remove reaction: %w", err)
	}

	r, _ := uc.repository.GetBySlug(ctx, slug)
	if r != nil {
		return uc.toResponse(r), nil
	}
	return &reaction.ReactionResponse{Slug: slug, Love: 0}, nil
}

func (uc *reactionUsecase) toResponse(r *models.Reaction) *reaction.ReactionResponse {
	return &reaction.ReactionResponse{
		Slug: r.Slug,
		Love: r.Love,
	}
}
