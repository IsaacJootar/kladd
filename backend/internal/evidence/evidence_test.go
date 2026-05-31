package evidence

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/google/uuid"
)

type recordingStore struct {
	record CreateRecord
	item   EvidenceItem
	items  []EvidenceItem
	err    error
}

func (store *recordingStore) Create(ctx context.Context, record CreateRecord) (EvidenceItem, error) {
	store.record = record
	if store.err != nil {
		return EvidenceItem{}, store.err
	}

	store.item = EvidenceItem{
		ID:          record.ID,
		Category:    record.Category,
		DisplayName: record.Metadata.DisplayName,
		FileName:    record.Metadata.OriginalFileName,
		ContentType: record.Metadata.ContentType,
		SizeBytes:   record.Metadata.SizeBytes,
		Status:      record.Status,
	}
	return store.item, nil
}

func (store *recordingStore) List(ctx context.Context, userID uuid.UUID) ([]EvidenceItem, error) {
	if store.err != nil {
		return nil, store.err
	}
	return store.items, nil
}

type recordingStorage struct {
	userID     uuid.UUID
	evidenceID uuid.UUID
	fileName   string
	content    string
	err        error
}

func (storage *recordingStorage) Save(ctx context.Context, userID uuid.UUID, evidenceID uuid.UUID, fileName string, reader io.Reader) (StoredObject, error) {
	storage.userID = userID
	storage.evidenceID = evidenceID
	storage.fileName = fileName
	content, err := io.ReadAll(reader)
	if err != nil {
		return StoredObject{}, err
	}
	storage.content = string(content)
	if storage.err != nil {
		return StoredObject{}, storage.err
	}

	return StoredObject{
		Path:     "evidence/" + userID.String() + "/" + evidenceID.String(),
		Provider: "local",
	}, nil
}

func TestServiceCreateStoresEvidenceMetadata(t *testing.T) {
	userID := uuid.New()
	store := &recordingStore{}
	storage := &recordingStorage{}
	service := NewService(store, storage)

	item, err := service.Create(context.Background(), CreateInput{
		UserID:      userID,
		Category:    " passport ",
		DisplayName: " Passport ",
		FileName:    "../passport.pdf",
		ContentType: "application/pdf",
		SizeBytes:   12,
		Reader:      bytes.NewBufferString("fake-content"),
	})
	if err != nil {
		t.Fatalf("create evidence: %v", err)
	}

	if storage.userID != userID {
		t.Fatalf("storage user = %s, want %s", storage.userID, userID)
	}

	if storage.fileName != "passport.pdf" {
		t.Fatalf("storage file name = %q, want passport.pdf", storage.fileName)
	}

	if storage.content != "fake-content" {
		t.Fatalf("storage content = %q, want fake-content", storage.content)
	}

	if store.record.FilePath == "" {
		t.Fatal("expected internal file path to be stored")
	}

	if item.DisplayName != "Passport" {
		t.Fatalf("display name = %q, want Passport", item.DisplayName)
	}

	if item.Status != StatusUploaded {
		t.Fatalf("status = %q, want %q", item.Status, StatusUploaded)
	}
}

func TestServiceCreateDefaultsDisplayName(t *testing.T) {
	store := &recordingStore{}
	service := NewService(store, &recordingStorage{})

	item, err := service.Create(context.Background(), CreateInput{
		UserID:    uuid.New(),
		Category:  "utility_bill",
		FileName:  "bill.pdf",
		SizeBytes: 9,
		Reader:    bytes.NewBufferString("fake"),
	})
	if err != nil {
		t.Fatalf("create evidence: %v", err)
	}

	if item.DisplayName != "utility_bill" {
		t.Fatalf("display name = %q, want category fallback", item.DisplayName)
	}
}

func TestServiceCreateValidatesInput(t *testing.T) {
	tests := []struct {
		name  string
		input CreateInput
		err   error
	}{
		{
			name: "missing category",
			input: CreateInput{
				UserID:    uuid.New(),
				FileName:  "passport.pdf",
				SizeBytes: 12,
				Reader:    bytes.NewBufferString("fake"),
			},
			err: ErrInvalidCategory,
		},
		{
			name: "missing file",
			input: CreateInput{
				UserID:   uuid.New(),
				Category: "passport",
			},
			err: ErrInvalidFile,
		},
	}

	service := NewService(&recordingStore{}, &recordingStorage{})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.Create(context.Background(), test.input)
			if !errors.Is(err, test.err) {
				t.Fatalf("error = %v, want %v", err, test.err)
			}
		})
	}
}

func TestEvidenceItemJSONHidesInternalFilePath(t *testing.T) {
	payload, err := json.Marshal(EvidenceItem{
		ID:          uuid.New(),
		Category:    "passport",
		DisplayName: "Passport",
		FileName:    "passport.pdf",
		Status:      StatusUploaded,
	})
	if err != nil {
		t.Fatalf("marshal evidence item: %v", err)
	}

	if bytes.Contains(payload, []byte("file_path")) {
		t.Fatal("evidence response exposed internal file path")
	}
}
