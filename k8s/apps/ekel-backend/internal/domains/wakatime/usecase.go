package wakatime

import "context"

type WakatimeUsecase interface {
	GetStats(ctx context.Context, rng string) (*StatsResponse, error)
}
