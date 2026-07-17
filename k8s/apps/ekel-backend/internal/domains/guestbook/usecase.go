package guestbook

import (
	"context"
)

type GuestbookUsecase interface {
	Create(ctx context.Context, req CreateGuestbookRequest) (*GuestbookResponse, error)
	GetByID(ctx context.Context, id string) (*GuestbookResponse, error)
	List(ctx context.Context) ([]*GuestbookResponse, error)
	Update(ctx context.Context, id string, req UpdateGuestbookRequest) (*GuestbookResponse, error)
	Delete(ctx context.Context, id string) error
}
