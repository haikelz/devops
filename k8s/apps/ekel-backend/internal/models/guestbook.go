package models

type GuestbookEntry struct {
	ID        string `gorm:"column:id;type:text;primaryKey" json:"id"`
	Username  string `gorm:"column:username;type:text" json:"username"`
	Message   string `gorm:"column:message;type:text" json:"message"`
	CreatedAt string `gorm:"column:created_at" json:"created_at"`
}

func (GuestbookEntry) TableName() string {
	return "guestbook"
}
