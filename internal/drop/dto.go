package drop

import (
	"time"

	"github.com/google/uuid"

	"unilo/internal/user"
)

type CreateDropRequest struct {
	Content string `json:"content"`
}

type DropResponse struct {
	ID           uuid.UUID `json:"id"`
	AuthorID     uuid.UUID `json:"author_id"`
	Author       user.DTO  `json:"author"`
	Content      string    `json:"content"`
	LikeCount    int       `json:"like_count"`
	CommentCount int       `json:"comment_count"`
	IsLikedByMe  bool      `json:"is_liked_by_me"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type DropDetailResponse struct {
	ID           uuid.UUID         `json:"id"`
	AuthorID     uuid.UUID         `json:"author_id"`
	Author       user.DTO          `json:"author"`
	Content      string            `json:"content"`
	LikeCount    int               `json:"like_count"`
	CommentCount int               `json:"comment_count"`
	IsLikedByMe  bool              `json:"is_liked_by_me"`
	Comments     []CommentResponse `json:"comments"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

type DropListResponse struct {
	Total int64          `json:"total"`
	Page  int            `json:"page"`
	Size  int            `json:"size"`
	Items []DropResponse `json:"items"`
}

type DeleteDropResponse struct {
	Deleted bool `json:"deleted"`
}

type LikeDropResponse struct {
	CurrentLikeCount int  `json:"current_like_count"`
	IsLiked          bool `json:"is_liked"`
}

type CreateCommentRequest struct {
	Content       string     `json:"content"`
	ParentID      *uuid.UUID `json:"parent_id"`
	ReplyToUserID *uuid.UUID `json:"reply_to_user_id"`
}

type CommentResponse struct {
	ID            uuid.UUID  `json:"id"`
	DropID        uuid.UUID  `json:"drop_id"`
	UserID        uuid.UUID  `json:"user_id"`
	Author        user.DTO   `json:"author"`
	ParentID      *uuid.UUID `json:"parent_id"`
	ReplyToUserID *uuid.UUID `json:"reply_to_user_id"`
	ReplyToUser   *user.DTO  `json:"reply_to_user,omitempty"`
	Content       string     `json:"content"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type DeleteCommentResponse struct {
	Deleted bool `json:"deleted"`
}

func ToDropResponse(d Drop, liked bool) DropResponse {
	return DropResponse{
		ID:           d.ID,
		AuthorID:     d.AuthorID,
		Author:       user.ToSenderDTO(d.Author),
		Content:      d.Content,
		LikeCount:    d.LikeCount,
		CommentCount: d.CommentCount,
		IsLikedByMe:  liked,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
	}
}

func ToDropDetailResponse(d Drop, liked bool, comments []CommentResponse) DropDetailResponse {
	return DropDetailResponse{
		ID:           d.ID,
		AuthorID:     d.AuthorID,
		Author:       user.ToSenderDTO(d.Author),
		Content:      d.Content,
		LikeCount:    d.LikeCount,
		CommentCount: d.CommentCount,
		IsLikedByMe:  liked,
		Comments:     comments,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
	}
}

func ToCommentResponse(c DropComment) CommentResponse {
	var replyToUser *user.DTO
	if c.ReplyToUser != nil {
		dto := user.ToSenderDTO(*c.ReplyToUser)
		replyToUser = &dto
	}
	return CommentResponse{
		ID:            c.ID,
		DropID:        c.DropID,
		UserID:        c.UserID,
		Author:        user.ToSenderDTO(c.Author),
		ParentID:      c.ParentID,
		ReplyToUserID: c.ReplyToUserID,
		ReplyToUser:   replyToUser,
		Content:       c.Content,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	}
}
