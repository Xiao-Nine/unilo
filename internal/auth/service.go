package auth

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"unilo/internal/config"
	"unilo/internal/user"
	"unilo/pkg/apperror"
	cryptoutil "unilo/pkg/crypto"
)

type Service struct {
	db         *gorm.DB
	users      *user.Repository
	tokens     *TokenManager
	app        config.AppConfig
	apiBaseURL string
}

func NewService(db *gorm.DB, users *user.Repository, tokens *TokenManager, cfg config.Config) *Service {
	return &Service{db: db, users: users, tokens: tokens, app: cfg.App, apiBaseURL: cfg.APIBaseURL()}
}

func (s *Service) TokenManager() *TokenManager {
	return s.tokens
}

func (s *Service) Verify(req VerifyRequest) (VerifyResponse, error) {
	if req.SecretKey == "" || req.SecretKey != s.app.ServiceSecret {
		return VerifyResponse{}, apperror.Forbidden("secret key is invalid")
	}
	return VerifyResponse{IsValid: true, ServerName: s.app.Name, ServerVersion: s.app.Version, APIBaseURL: s.apiBaseURL}, nil
}

func (s *Service) Register(req RegisterRequest) (RegisterResponse, error) {
	username := strings.TrimSpace(req.Username)
	nickname := strings.TrimSpace(req.Nickname)
	if len(username) < 3 || len(username) > 50 {
		return RegisterResponse{}, apperror.BadRequest("username length must be between 3 and 50")
	}
	if len(req.Password) < 8 {
		return RegisterResponse{}, apperror.BadRequest("password length must be at least 8")
	}
	if nickname == "" || len(nickname) > 50 {
		return RegisterResponse{}, apperror.BadRequest("nickname length must be between 1 and 50")
	}

	if _, err := s.users.FindByUsername(username); err == nil {
		return RegisterResponse{}, apperror.Conflict("username already exists")
	} else if !user.IsNotFound(err) {
		return RegisterResponse{}, apperror.Internal(err)
	}

	passwordHash, err := cryptoutil.HashPassword(req.Password)
	if err != nil {
		return RegisterResponse{}, apperror.Internal(err)
	}

	u := user.User{Username: username, PasswordHash: passwordHash, Nickname: nickname}
	if err := s.users.Create(&u); err != nil {
		return RegisterResponse{}, apperror.Internal(err)
	}

	return RegisterResponse{User: user.ToDTO(u)}, nil
}

func (s *Service) Login(req LoginRequest) (LoginResponse, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" || req.Password == "" {
		return LoginResponse{}, apperror.BadRequest("username and password are required")
	}

	u, err := s.users.FindByUsername(username)
	if err != nil {
		if user.IsNotFound(err) {
			return LoginResponse{}, apperror.Unauthorized("username or password is invalid")
		}
		return LoginResponse{}, apperror.Internal(err)
	}
	if !cryptoutil.VerifyPassword(u.PasswordHash, req.Password) {
		return LoginResponse{}, apperror.Unauthorized("username or password is invalid")
	}

	tokenResp, err := s.issueTokens(u.ID)
	if err != nil {
		return LoginResponse{}, err
	}

	return LoginResponse{
		AccessToken:           tokenResp.AccessToken,
		RefreshToken:          tokenResp.RefreshToken,
		AccessTokenExpiresAt:  tokenResp.AccessTokenExpiresAt,
		RefreshTokenExpiresAt: tokenResp.RefreshTokenExpiresAt,
		User:                  user.ToDTO(u),
	}, nil
}

func (s *Service) Refresh(req RefreshRequest) (TokenResponse, error) {
	if req.RefreshToken == "" {
		return TokenResponse{}, apperror.BadRequest("refresh_token is required")
	}
	claims, err := s.tokens.VerifyRefresh(req.RefreshToken)
	if err != nil {
		return TokenResponse{}, apperror.Unauthorized("refresh token is invalid")
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return TokenResponse{}, apperror.Unauthorized("refresh token is invalid")
	}

	var resp TokenResponse
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var existing RefreshToken
		tokenHash := cryptoutil.HashToken(req.RefreshToken)
		err := tx.Where("token_hash = ? AND user_id = ? AND revoked_at IS NULL AND expires_at > ?", tokenHash, userID, time.Now().UTC()).First(&existing).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperror.Unauthorized("refresh token is invalid")
			}
			return apperror.Internal(err)
		}

		now := time.Now().UTC()
		if err := tx.Model(&existing).Update("revoked_at", now).Error; err != nil {
			return apperror.Internal(err)
		}

		issued, err := s.issueTokensWithDB(tx, userID)
		if err != nil {
			return err
		}
		resp = issued
		return nil
	})
	if err != nil {
		return TokenResponse{}, err
	}

	return resp, nil
}

func (s *Service) Logout(req LogoutRequest) (LogoutResponse, error) {
	if req.RefreshToken == "" {
		return LogoutResponse{}, apperror.BadRequest("refresh_token is required")
	}
	claims, err := s.tokens.VerifyRefresh(req.RefreshToken)
	if err != nil {
		return LogoutResponse{}, apperror.Unauthorized("refresh token is invalid")
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return LogoutResponse{}, apperror.Unauthorized("refresh token is invalid")
	}

	now := time.Now().UTC()
	tokenHash := cryptoutil.HashToken(req.RefreshToken)
	if err := s.db.Model(&RefreshToken{}).Where("token_hash = ? AND user_id = ? AND revoked_at IS NULL", tokenHash, userID).Update("revoked_at", now).Error; err != nil {
		return LogoutResponse{}, apperror.Internal(err)
	}
	return LogoutResponse{LoggedOut: true}, nil
}

func (s *Service) Me(userID uuid.UUID) (user.DTO, error) {
	u, err := s.users.FindByID(userID)
	if err != nil {
		if user.IsNotFound(err) {
			return user.DTO{}, apperror.Unauthorized("user is not found")
		}
		return user.DTO{}, apperror.Internal(err)
	}
	return user.ToDTO(u), nil
}

func (s *Service) issueTokens(userID uuid.UUID) (TokenResponse, error) {
	return s.issueTokensWithDB(s.db, userID)
}

func (s *Service) issueTokensWithDB(db *gorm.DB, userID uuid.UUID) (TokenResponse, error) {
	accessToken, accessExpiresAt, err := s.tokens.IssueAccess(userID)
	if err != nil {
		return TokenResponse{}, apperror.Internal(err)
	}
	refreshToken, refreshExpiresAt, err := s.tokens.IssueRefresh(userID)
	if err != nil {
		return TokenResponse{}, apperror.Internal(err)
	}

	record := RefreshToken{UserID: userID, TokenHash: cryptoutil.HashToken(refreshToken), ExpiresAt: refreshExpiresAt}
	if err := db.Create(&record).Error; err != nil {
		return TokenResponse{}, apperror.Internal(err)
	}

	return TokenResponse{AccessToken: accessToken, RefreshToken: refreshToken, AccessTokenExpiresAt: accessExpiresAt, RefreshTokenExpiresAt: refreshExpiresAt}, nil
}
