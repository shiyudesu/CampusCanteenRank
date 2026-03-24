package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"CampusCanteenRank/server/internal/dto/auth"
	"CampusCanteenRank/server/internal/model/auth"
	authpkg "CampusCanteenRank/server/internal/pkg/auth"
	errpkg "CampusCanteenRank/server/internal/pkg/errors"
	"CampusCanteenRank/server/internal/repository/auth"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	users         repository.UserRepository
	refreshTokens repository.RefreshTokenRepository
	secret        string
	issuer        string
	accessTTL     time.Duration
	refreshTTL    time.Duration
	nowFunc       func() time.Time
	idGen         func() string
}

func NewAuthService(
	users repository.UserRepository,
	refreshTokens repository.RefreshTokenRepository,
	secret string,
) *AuthService {
	return &AuthService{
		users:         users,
		refreshTokens: refreshTokens,
		secret:        secret,
		issuer:        "canteen-api",
		accessTTL:     2 * time.Hour,
		refreshTTL:    7 * 24 * time.Hour,
		nowFunc:       time.Now,
		idGen:         defaultID,
	}
}

func (s *AuthService) Register(ctx context.Context, req dto.RegisterRequest) (int64, error) {
	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Nickname) == "" || strings.TrimSpace(req.Password) == "" {
		return 0, errpkg.New(errpkg.CodeBadRequest, "invalid params", nil)
	}

	if _, err := s.users.GetByEmail(ctx, req.Email); err == nil {
		return 0, errpkg.New(errpkg.CodeConflict, "email already exists", nil)
	} else if !errors.Is(err, repository.ErrNotFound) {
		return 0, errpkg.New(errpkg.CodeInternal, "internal error", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return 0, errpkg.New(errpkg.CodeInternal, "internal error", err)
	}

	user := &model.User{
		Email:        req.Email,
		Nickname:     req.Nickname,
		PasswordHash: string(hash),
		Status:       1,
	}
	if err := s.users.Create(ctx, user); err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			return 0, errpkg.New(errpkg.CodeConflict, "email already exists", nil)
		}
		return 0, errpkg.New(errpkg.CodeInternal, "internal error", err)
	}

	return user.ID, nil
}

func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginData, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		deviceID = "default"
	}

	user, err := s.users.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errpkg.New(errpkg.CodeUnauthorized, "invalid credentials", nil)
		}
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", err)
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		return nil, errpkg.New(errpkg.CodeUnauthorized, "invalid credentials", nil)
	}

	accessToken, expiresIn, err := s.buildAccessToken(user.ID)
	if err != nil {
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", err)
	}
	refreshToken, jti, refreshExpireAt, err := s.buildRefreshToken(user.ID, deviceID)
	if err != nil {
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", err)
	}

	if err := s.refreshTokens.Save(ctx, repository.RefreshTokenRecord{UserID: user.ID, TokenJTI: jti, DeviceID: deviceID, ExpiredAt: refreshExpireAt}); err != nil {
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", err)
	}

	return &dto.LoginData{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
		User: dto.LoginUserVO{
			ID:       user.ID,
			Nickname: user.Nickname,
		},
	}, nil
}

func (s *AuthService) Refresh(ctx context.Context, req dto.RefreshRequest) (*dto.RefreshData, error) {
	deviceID := strings.TrimSpace(req.DeviceID)

	claims, err := authpkg.ParseToken(s.secret, req.RefreshToken)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errpkg.New(errpkg.CodeUnauthorized, "token expired", nil)
		}
		return nil, errpkg.New(errpkg.CodeUnauthorized, "invalid token", nil)
	}
	if claims.TokenType != authpkg.TokenTypeRefresh || claims.UserID <= 0 || claims.JTI == "" {
		return nil, errpkg.New(errpkg.CodeUnauthorized, "invalid token", nil)
	}
	if deviceID != "" && claims.DeviceID != "" && deviceID != claims.DeviceID {
		return nil, errpkg.New(errpkg.CodeUnauthorized, "invalid token", nil)
	}
	if claims.DeviceID == "" {
		claims.DeviceID = "default"
	}

	if err := s.refreshTokens.Consume(ctx, claims.UserID, claims.JTI, claims.DeviceID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errpkg.New(errpkg.CodeUnauthorized, "invalid token", nil)
		}
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", err)
	}

	accessToken, expiresIn, err := s.buildAccessToken(claims.UserID)
	if err != nil {
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", err)
	}
	newRefreshToken, newJTI, refreshExpireAt, err := s.buildRefreshToken(claims.UserID, claims.DeviceID)
	if err != nil {
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", err)
	}
	if err := s.refreshTokens.Save(ctx, repository.RefreshTokenRecord{UserID: claims.UserID, TokenJTI: newJTI, DeviceID: claims.DeviceID, ExpiredAt: refreshExpireAt}); err != nil {
		return nil, errpkg.New(errpkg.CodeInternal, "internal error", err)
	}

	return &dto.RefreshData{AccessToken: accessToken, RefreshToken: newRefreshToken, ExpiresIn: expiresIn}, nil
}

func (s *AuthService) buildAccessToken(userID int64) (string, int64, error) {
	now := s.nowFunc().UTC()
	exp := now.Add(s.accessTTL)
	claims := authpkg.Claims{
		UserID:    userID,
		TokenType: authpkg.TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	token, err := authpkg.SignToken(s.secret, claims)
	if err != nil {
		return "", 0, err
	}
	return token, int64(s.accessTTL.Seconds()), nil
}

func (s *AuthService) buildRefreshToken(userID int64, deviceID string) (string, string, time.Time, error) {
	now := s.nowFunc().UTC()
	exp := now.Add(s.refreshTTL)
	jti := s.idGen()
	claims := authpkg.Claims{
		UserID:    userID,
		TokenType: authpkg.TokenTypeRefresh,
		JTI:       jti,
		DeviceID:  deviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        jti,
		},
	}
	token, err := authpkg.SignToken(s.secret, claims)
	if err != nil {
		return "", "", time.Time{}, err
	}
	return token, jti, exp, nil
}

func (s *AuthService) Logout(ctx context.Context, req dto.RefreshRequest) error {
	claims, err := authpkg.ParseToken(s.secret, req.RefreshToken)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return errpkg.New(errpkg.CodeUnauthorized, "token expired", nil)
		}
		return errpkg.New(errpkg.CodeUnauthorized, "invalid token", nil)
	}
	if claims.TokenType != authpkg.TokenTypeRefresh || claims.UserID <= 0 || claims.JTI == "" {
		return errpkg.New(errpkg.CodeUnauthorized, "invalid token", nil)
	}
	deviceID := claims.DeviceID
	if deviceID == "" {
		deviceID = strings.TrimSpace(req.DeviceID)
	}
	if deviceID == "" {
		deviceID = "default"
	}
	if err := s.refreshTokens.Consume(ctx, claims.UserID, claims.JTI, deviceID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return errpkg.New(errpkg.CodeUnauthorized, "invalid token", nil)
		}
		return errpkg.New(errpkg.CodeInternal, "internal error", err)
	}
	return nil
}

func defaultID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
