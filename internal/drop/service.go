package drop

import (
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"unilo/internal/user"
	"unilo/pkg/apperror"
)

const (
	maxDropContentLength    = 20000
	maxCommentContentLength = 5000
)

type Indexer interface {
	UpsertDocument(tx *gorm.DB, sourceType string, sourceID string, title string, content string, metadata map[string]any) error
	DeleteDocument(tx *gorm.DB, sourceType string, sourceID string) error
}

type noopIndexer struct{}

func (noopIndexer) UpsertDocument(*gorm.DB, string, string, string, string, map[string]any) error {
	return nil
}

func (noopIndexer) DeleteDocument(*gorm.DB, string, string) error {
	return nil
}

type Service struct {
	repo    *Repository
	indexer Indexer
}

func NewService(repo *Repository, indexer Indexer) *Service {
	if indexer == nil {
		indexer = noopIndexer{}
	}
	return &Service{repo: repo, indexer: indexer}
}

func (s *Service) ListDrops(userID uuid.UUID, page int, size int) (DropListResponse, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	drops, total, err := s.repo.ListDrops(page, size)
	if err != nil {
		return DropListResponse{}, apperror.Internal(err)
	}
	liked, err := s.likedDropIDs(userID, drops)
	if err != nil {
		return DropListResponse{}, apperror.Internal(err)
	}

	items := make([]DropResponse, 0, len(drops))
	for _, d := range drops {
		items = append(items, ToDropResponse(d, liked[d.ID]))
	}
	return DropListResponse{Total: total, Page: page, Size: size, Items: items}, nil
}

func (s *Service) CreateDrop(userID uuid.UUID, req CreateDropRequest) (DropResponse, error) {
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return DropResponse{}, apperror.BadRequest("content is required")
	}
	if len(content) > maxDropContentLength {
		return DropResponse{}, apperror.BadRequest("content is too long")
	}

	d := Drop{AuthorID: userID, Content: content}
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&d).Error; err != nil {
			return err
		}
		if err := tx.Preload("Author").First(&d, "id = ?", d.ID).Error; err != nil {
			return err
		}
		return s.indexer.UpsertDocument(tx, "drop", d.ID.String(), "动态", d.Content, map[string]any{"author_id": d.AuthorID})
	}); err != nil {
		return DropResponse{}, apperror.Internal(err)
	}
	return ToDropResponse(d, false), nil
}

func (s *Service) GetDrop(userID uuid.UUID, dropID uuid.UUID) (DropDetailResponse, error) {
	d, err := s.repo.FindDrop(dropID)
	if err != nil {
		if IsNotFound(err) {
			return DropDetailResponse{}, apperror.NotFound("drop is not found")
		}
		return DropDetailResponse{}, apperror.Internal(err)
	}
	liked, err := s.isDropLiked(userID, dropID)
	if err != nil {
		return DropDetailResponse{}, apperror.Internal(err)
	}
	comments, err := s.repo.ListComments(dropID)
	if err != nil {
		return DropDetailResponse{}, apperror.Internal(err)
	}

	respComments := make([]CommentResponse, 0, len(comments))
	for _, c := range comments {
		respComments = append(respComments, ToCommentResponse(c))
	}
	return ToDropDetailResponse(d, liked, respComments), nil
}

func (s *Service) DeleteDrop(userID uuid.UUID, dropID uuid.UUID) (DeleteDropResponse, error) {
	d, err := s.repo.FindDrop(dropID)
	if err != nil {
		if IsNotFound(err) {
			return DeleteDropResponse{}, apperror.NotFound("drop is not found")
		}
		return DeleteDropResponse{}, apperror.Internal(err)
	}
	if d.AuthorID != userID {
		return DeleteDropResponse{}, apperror.Forbidden("forbidden")
	}
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&d).Error; err != nil {
			return err
		}
		return s.indexer.DeleteDocument(tx, "drop", dropID.String())
	}); err != nil {
		return DeleteDropResponse{}, apperror.Internal(err)
	}
	return DeleteDropResponse{Deleted: true}, nil
}

func (s *Service) ToggleLike(userID uuid.UUID, dropID uuid.UUID) (LikeDropResponse, error) {
	var resp LikeDropResponse
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		var d Drop
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&d, "id = ?", dropID).Error; err != nil {
			if IsNotFound(err) {
				return apperror.NotFound("drop is not found")
			}
			return apperror.Internal(err)
		}

		var like DropLike
		err := tx.Where("drop_id = ? AND user_id = ?", dropID, userID).First(&like).Error
		if err == nil {
			if err := tx.Delete(&like).Error; err != nil {
				return apperror.Internal(err)
			}
			if err := tx.Model(&Drop{}).Where("id = ?", dropID).Update("like_count", gorm.Expr("GREATEST(like_count - 1, 0)")).Error; err != nil {
				return apperror.Internal(err)
			}
			count := d.LikeCount - 1
			if count < 0 {
				count = 0
			}
			resp = LikeDropResponse{CurrentLikeCount: count, IsLiked: false}
			return nil
		}
		if !IsNotFound(err) {
			return apperror.Internal(err)
		}

		like = DropLike{DropID: dropID, UserID: userID}
		if err := tx.Create(&like).Error; err != nil {
			return apperror.Internal(err)
		}
		if err := tx.Model(&Drop{}).Where("id = ?", dropID).Update("like_count", gorm.Expr("like_count + 1")).Error; err != nil {
			return apperror.Internal(err)
		}
		resp = LikeDropResponse{CurrentLikeCount: d.LikeCount + 1, IsLiked: true}
		return nil
	})
	if err != nil {
		return LikeDropResponse{}, err
	}
	return resp, nil
}

func (s *Service) CreateComment(userID uuid.UUID, dropID uuid.UUID, req CreateCommentRequest) (CommentResponse, error) {
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return CommentResponse{}, apperror.BadRequest("content is required")
	}
	if len(content) > maxCommentContentLength {
		return CommentResponse{}, apperror.BadRequest("content is too long")
	}

	var resp CommentResponse
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&Drop{}, "id = ?", dropID).Error; err != nil {
			if IsNotFound(err) {
				return apperror.NotFound("drop is not found")
			}
			return apperror.Internal(err)
		}
		if req.ParentID != nil {
			var parent DropComment
			if err := tx.First(&parent, "id = ? AND drop_id = ?", *req.ParentID, dropID).Error; err != nil {
				if IsNotFound(err) {
					return apperror.BadRequest("parent_id is invalid")
				}
				return apperror.Internal(err)
			}
		}
		if req.ReplyToUserID != nil {
			var replyTo user.User
			if err := tx.First(&replyTo, "id = ?", *req.ReplyToUserID).Error; err != nil {
				if IsNotFound(err) {
					return apperror.BadRequest("reply_to_user_id is invalid")
				}
				return apperror.Internal(err)
			}
		}

		comment := DropComment{DropID: dropID, UserID: userID, ParentID: req.ParentID, ReplyToUserID: req.ReplyToUserID, Content: content}
		if err := tx.Create(&comment).Error; err != nil {
			return apperror.Internal(err)
		}
		if err := tx.Model(&Drop{}).Where("id = ?", dropID).Update("comment_count", gorm.Expr("comment_count + 1")).Error; err != nil {
			return apperror.Internal(err)
		}
		if err := tx.Preload("Author").Preload("ReplyToUser").First(&comment, "id = ?", comment.ID).Error; err != nil {
			return apperror.Internal(err)
		}
		resp = ToCommentResponse(comment)
		return nil
	})
	if err != nil {
		return CommentResponse{}, err
	}
	return resp, nil
}

func (s *Service) DeleteComment(userID uuid.UUID, dropID uuid.UUID, commentID uuid.UUID) (DeleteCommentResponse, error) {
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&Drop{}, "id = ?", dropID).Error; err != nil {
			if IsNotFound(err) {
				return apperror.NotFound("drop is not found")
			}
			return apperror.Internal(err)
		}

		var comment DropComment
		if err := tx.First(&comment, "id = ? AND drop_id = ?", commentID, dropID).Error; err != nil {
			if IsNotFound(err) {
				return apperror.NotFound("comment is not found")
			}
			return apperror.Internal(err)
		}
		if comment.UserID != userID {
			return apperror.Forbidden("forbidden")
		}
		if err := tx.Delete(&comment).Error; err != nil {
			return apperror.Internal(err)
		}
		if err := tx.Model(&Drop{}).Where("id = ?", dropID).Update("comment_count", gorm.Expr("GREATEST(comment_count - 1, 0)")).Error; err != nil {
			return apperror.Internal(err)
		}
		return nil
	})
	if err != nil {
		return DeleteCommentResponse{}, err
	}
	return DeleteCommentResponse{Deleted: true}, nil
}

func (s *Service) likedDropIDs(userID uuid.UUID, drops []Drop) (map[uuid.UUID]bool, error) {
	liked := make(map[uuid.UUID]bool)
	if len(drops) == 0 {
		return liked, nil
	}
	ids := make([]uuid.UUID, 0, len(drops))
	for _, d := range drops {
		ids = append(ids, d.ID)
	}

	var likes []DropLike
	if err := s.repo.DB().Where("user_id = ? AND drop_id IN ?", userID, ids).Find(&likes).Error; err != nil {
		return nil, err
	}
	for _, like := range likes {
		liked[like.DropID] = true
	}
	return liked, nil
}

func (s *Service) isDropLiked(userID uuid.UUID, dropID uuid.UUID) (bool, error) {
	var count int64
	err := s.repo.DB().Model(&DropLike{}).Where("user_id = ? AND drop_id = ?", userID, dropID).Count(&count).Error
	return count > 0, err
}

func ParseDropID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, apperror.BadRequest("drop_id is invalid")
	}
	return id, nil
}

func ParseCommentID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, apperror.BadRequest("comment_id is invalid")
	}
	return id, nil
}
