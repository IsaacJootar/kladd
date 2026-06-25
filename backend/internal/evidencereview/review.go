package evidencereview

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	StatusVerified = "verified"
	StatusRejected = "rejected"
)

var (
	ErrInvalidEmail      = errors.New("valid user email is required")
	ErrInvalidEvidenceID = errors.New("evidence_id is required")
	ErrInvalidStatus     = errors.New("status must be verified or rejected")
	ErrEvidenceNotFound  = errors.New("evidence item not found")
)

type ReviewInput struct {
	UserEmail  string
	EvidenceID uuid.UUID
	Status     string
}

type ReviewResult struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	UserEmail   string    `json:"user_email"`
	Category    string    `json:"category"`
	DisplayName string    `json:"display_name"`
	Status      string    `json:"status"`
	ReviewedAt  time.Time `json:"reviewed_at"`
}

type Store interface {
	Review(ctx context.Context, input ReviewInput, reviewedAt time.Time) (ReviewResult, error)
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) Service {
	return Service{
		store: store,
		now:   time.Now,
	}
}

func NewServiceWithClock(store Store, now func() time.Time) Service {
	return Service{
		store: store,
		now:   now,
	}
}

func (service Service) Review(ctx context.Context, input ReviewInput) (ReviewResult, error) {
	cleaned, err := cleanInput(input)
	if err != nil {
		return ReviewResult{}, err
	}

	return service.store.Review(ctx, cleaned, service.now().UTC())
}

func cleanInput(input ReviewInput) (ReviewInput, error) {
	email := strings.ToLower(strings.TrimSpace(input.UserEmail))
	if _, err := mail.ParseAddress(email); err != nil {
		return ReviewInput{}, ErrInvalidEmail
	}
	if input.EvidenceID == uuid.Nil {
		return ReviewInput{}, ErrInvalidEvidenceID
	}

	status := strings.ToLower(strings.TrimSpace(input.Status))
	if status != StatusVerified && status != StatusRejected {
		return ReviewInput{}, ErrInvalidStatus
	}

	input.UserEmail = email
	input.Status = status
	return input, nil
}
