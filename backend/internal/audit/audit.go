package audit

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidUser = errors.New("user_id is required")
)

type Event struct {
	ID          uuid.UUID `json:"id"`
	EventType   string    `json:"event_type"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type Record struct {
	ID        uuid.UUID
	EventType string
	Metadata  map[string]string
	CreatedAt time.Time
}

type Store interface {
	ListForUser(ctx context.Context, userID uuid.UUID, limit int) ([]Record, error)
}

type Service struct {
	store Store
	limit int
}

func NewService(store Store) Service {
	return Service{
		store: store,
		limit: 20,
	}
}

func (service Service) ListForUser(ctx context.Context, userID uuid.UUID) ([]Event, error) {
	if userID == uuid.Nil {
		return nil, ErrInvalidUser
	}

	records, err := service.store.ListForUser(ctx, userID, service.limit)
	if err != nil {
		return nil, err
	}

	events := make([]Event, 0, len(records))
	for _, record := range records {
		events = append(events, formatEvent(record))
	}

	return events, nil
}

func formatEvent(record Record) Event {
	title, description := eventCopy(record.EventType, record.Metadata)

	return Event{
		ID:          record.ID,
		EventType:   record.EventType,
		Title:       title,
		Description: description,
		CreatedAt:   record.CreatedAt,
	}
}

func eventCopy(eventType string, metadata map[string]string) (string, string) {
	switch eventType {
	case "user.created":
		return "Account created", "Your Kladd account was created."
	case "user.login":
		return "Signed in", "Your account was signed in."
	case "security_pin.set":
		return "Security PIN set", "Your Security PIN was set."
	case "security_pin.reset":
		return "Security PIN reset", "Your Security PIN was reset after account confirmation."
	case "security_pin.validation_failed":
		return "Security PIN check failed", "A Security PIN approval attempt did not pass."
	case "security_pin.validation_succeeded":
		return "Security PIN confirmed", "A Security PIN approval check passed."
	case "evidence.created":
		return "Record added", "A " + friendlyValue(metadata["category"]) + " record was added."
	case "claim_request.approved":
		return "Proof request approved", "A proof request was approved with your Security PIN."
	case "claim_request.denied":
		return "Proof request denied", "A proof request was denied. No proof was released."
	case "claim.revoked":
		return "Proof revoked", "An active proof was revoked and its proof details are hidden."
	default:
		return "Activity recorded", "A Kladd account activity was recorded."
	}
}

func friendlyValue(value string) string {
	cleaned := strings.TrimSpace(value)
	if cleaned == "" {
		return "new"
	}

	return strings.ReplaceAll(cleaned, "_", " ")
}
