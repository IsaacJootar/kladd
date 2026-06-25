package evidencereview

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type recordingStore struct {
	input      ReviewInput
	reviewedAt time.Time
	result     ReviewResult
	err        error
}

func (store *recordingStore) Review(ctx context.Context, input ReviewInput, reviewedAt time.Time) (ReviewResult, error) {
	store.input = input
	store.reviewedAt = reviewedAt
	if store.err != nil {
		return ReviewResult{}, store.err
	}
	if store.result.ID != uuid.Nil {
		return store.result, nil
	}
	return ReviewResult{
		ID:         input.EvidenceID,
		UserEmail:  input.UserEmail,
		Status:     input.Status,
		ReviewedAt: reviewedAt,
	}, nil
}

func TestServiceReviewCleansAndStoresReview(t *testing.T) {
	evidenceID := uuid.New()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	store := &recordingStore{}
	service := NewServiceWithClock(store, func() time.Time { return now })

	result, err := service.Review(context.Background(), ReviewInput{
		UserEmail:  " Zenith@Gmail.com ",
		EvidenceID: evidenceID,
		Status:     " Verified ",
	})
	if err != nil {
		t.Fatalf("review evidence: %v", err)
	}

	if store.input.UserEmail != "zenith@gmail.com" {
		t.Fatalf("email = %q, want normalized email", store.input.UserEmail)
	}
	if store.input.Status != StatusVerified {
		t.Fatalf("status = %q, want %q", store.input.Status, StatusVerified)
	}
	if !store.reviewedAt.Equal(now) {
		t.Fatalf("reviewed at = %s, want %s", store.reviewedAt, now)
	}
	if result.ID != evidenceID {
		t.Fatalf("evidence id = %s, want %s", result.ID, evidenceID)
	}
}

func TestServiceReviewValidatesInput(t *testing.T) {
	tests := []struct {
		name  string
		input ReviewInput
		err   error
	}{
		{name: "bad email", input: ReviewInput{UserEmail: "bad", EvidenceID: uuid.New(), Status: StatusVerified}, err: ErrInvalidEmail},
		{name: "missing evidence", input: ReviewInput{UserEmail: "ada@example.com", Status: StatusVerified}, err: ErrInvalidEvidenceID},
		{name: "bad status", input: ReviewInput{UserEmail: "ada@example.com", EvidenceID: uuid.New(), Status: "pending"}, err: ErrInvalidStatus},
	}

	service := NewService(&recordingStore{})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.Review(context.Background(), test.input)
			if !errors.Is(err, test.err) {
				t.Fatalf("error = %v, want %v", err, test.err)
			}
		})
	}
}

func TestServiceReviewReturnsStoreError(t *testing.T) {
	storeErr := errors.New("store failed")
	service := NewService(&recordingStore{err: storeErr})

	_, err := service.Review(context.Background(), ReviewInput{
		UserEmail:  "ada@example.com",
		EvidenceID: uuid.New(),
		Status:     StatusRejected,
	})
	if !errors.Is(err, storeErr) {
		t.Fatalf("error = %v, want %v", err, storeErr)
	}
}
