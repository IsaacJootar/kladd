package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	TokenTypeBearer = "Bearer"
	DefaultTokenTTL = time.Hour
)

var (
	ErrInvalidToken = errors.New("invalid access token")
)

type TokenManager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewTokenManager(secret string, ttl time.Duration) TokenManager {
	return NewTokenManagerWithClock(secret, ttl, time.Now)
}

func NewTokenManagerWithClock(secret string, ttl time.Duration, now func() time.Time) TokenManager {
	if ttl <= 0 {
		ttl = DefaultTokenTTL
	}

	return TokenManager{
		secret: []byte(secret),
		ttl:    ttl,
		now:    now,
	}
}

func (manager TokenManager) Issue(userID uuid.UUID) (string, time.Time, error) {
	issuedAt := manager.now()
	expiresAt := issuedAt.Add(manager.ttl)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	})

	tokenString, err := token.SignedString(manager.secret)
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiresAt, nil
}

func (manager TokenManager) Verify(tokenString string) (uuid.UUID, error) {
	claims := jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}

		return manager.secret, nil
	}, jwt.WithExpirationRequired(), jwt.WithTimeFunc(manager.now))
	if err != nil || !token.Valid {
		return uuid.Nil, ErrInvalidToken
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}

	return userID, nil
}
