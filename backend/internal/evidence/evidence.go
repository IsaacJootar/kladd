package evidence

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	StatusUploaded = "uploaded"
)

var (
	ErrInvalidCategory = errors.New("evidence category is required")
	ErrInvalidFile     = errors.New("evidence file is required")
)

type CreateInput struct {
	UserID      uuid.UUID
	Category    string
	DisplayName string
	FileName    string
	ContentType string
	SizeBytes   int64
	Reader      io.Reader
}

type EvidenceItem struct {
	ID          uuid.UUID `json:"id"`
	Category    string    `json:"category"`
	DisplayName string    `json:"display_name"`
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	Status      string    `json:"status"`
	UploadedAt  time.Time `json:"uploaded_at"`
}

type Metadata struct {
	DisplayName      string `json:"display_name"`
	OriginalFileName string `json:"original_file_name"`
	ContentType      string `json:"content_type"`
	SizeBytes        int64  `json:"size_bytes"`
	StorageProvider  string `json:"storage_provider"`
}

type CreateRecord struct {
	ID       uuid.UUID
	UserID   uuid.UUID
	Category string
	FilePath string
	Status   string
	Metadata Metadata
}

type StoredObject struct {
	Path     string
	Provider string
}

type Storage interface {
	Save(ctx context.Context, userID uuid.UUID, evidenceID uuid.UUID, fileName string, reader io.Reader) (StoredObject, error)
}

type Store interface {
	Create(ctx context.Context, record CreateRecord) (EvidenceItem, error)
	List(ctx context.Context, userID uuid.UUID) ([]EvidenceItem, error)
}

type Service struct {
	store   Store
	storage Storage
}

func NewService(store Store, storage Storage) Service {
	return Service{
		store:   store,
		storage: storage,
	}
}

func (service Service) Create(ctx context.Context, input CreateInput) (EvidenceItem, error) {
	record, err := service.prepareRecord(ctx, input)
	if err != nil {
		return EvidenceItem{}, err
	}

	return service.store.Create(ctx, record)
}

func (service Service) List(ctx context.Context, userID uuid.UUID) ([]EvidenceItem, error) {
	return service.store.List(ctx, userID)
}

func (service Service) prepareRecord(ctx context.Context, input CreateInput) (CreateRecord, error) {
	category := strings.TrimSpace(input.Category)
	if category == "" {
		return CreateRecord{}, ErrInvalidCategory
	}

	if input.Reader == nil || strings.TrimSpace(input.FileName) == "" || input.SizeBytes <= 0 {
		return CreateRecord{}, ErrInvalidFile
	}

	evidenceID := uuid.New()
	fileName := filepath.Base(input.FileName)
	stored, err := service.storage.Save(ctx, input.UserID, evidenceID, fileName, input.Reader)
	if err != nil {
		return CreateRecord{}, err
	}

	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = category
	}

	return CreateRecord{
		ID:       evidenceID,
		UserID:   input.UserID,
		Category: category,
		FilePath: stored.Path,
		Status:   StatusUploaded,
		Metadata: Metadata{
			DisplayName:      displayName,
			OriginalFileName: fileName,
			ContentType:      strings.TrimSpace(input.ContentType),
			SizeBytes:        input.SizeBytes,
			StorageProvider:  stored.Provider,
		},
	}, nil
}
