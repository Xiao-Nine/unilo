package user

import (
	"time"

	"github.com/google/uuid"
)

type DTO struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username,omitempty"`
	Nickname  string    `json:"nickname"`
	AvatarURL string    `json:"avatar_url"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

func ToDTO(u User) DTO {
	return DTO{
		ID:        u.ID,
		Username:  u.Username,
		Nickname:  u.Nickname,
		AvatarURL: u.AvatarURL,
		CreatedAt: u.CreatedAt,
	}
}

func ToSenderDTO(u User) DTO {
	return DTO{
		ID:        u.ID,
		Nickname:  u.Nickname,
		AvatarURL: u.AvatarURL,
	}
}
