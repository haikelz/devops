package reaction

import "context"

type ReactionUsecase interface {
	List(ctx context.Context) ([]*ReactionResponse, error)
	Add(ctx context.Context, slug string) (*ReactionResponse, error)
	Remove(ctx context.Context, slug string) (*ReactionResponse, error)
}
