package user

import "time"

type User struct {
	ID           int64
	TelegramID   int64
	Username     string
	FirstName    string
	LanguageCode string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
