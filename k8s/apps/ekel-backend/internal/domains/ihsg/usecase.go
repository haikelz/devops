package ihsg

import "context"

type IhsgUsecase interface {
	GetMarkets(ctx context.Context) ([]MarketDataset, error)
}
