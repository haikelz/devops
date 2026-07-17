package guestbook

type CreateGuestbookRequest struct {
	Username string `json:"username" validate:"required,min=2,max=255"`
	Message  string `json:"message" validate:"required,min=10,max=2000"`
}

type UpdateGuestbookRequest struct {
	Username string `json:"username" validate:"required,min=2,max=255"`
	Message  string `json:"message" validate:"required,min=10,max=2000"`
}

type GuestbookResponse struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}
