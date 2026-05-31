package evidence

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

type LocalStorage struct {
	root string
}

func NewLocalStorage(root string) LocalStorage {
	return LocalStorage{root: root}
}

func (storage LocalStorage) Save(ctx context.Context, userID uuid.UUID, evidenceID uuid.UUID, fileName string, reader io.Reader) (StoredObject, error) {
	select {
	case <-ctx.Done():
		return StoredObject{}, ctx.Err()
	default:
	}

	relativePath := filepath.Join("evidence", userID.String(), evidenceID.String()+"-"+safeFileName(fileName))
	fullPath := filepath.Join(storage.root, relativePath)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
		return StoredObject{}, err
	}

	file, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return StoredObject{}, err
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return StoredObject{}, err
	}

	return StoredObject{
		Path:     relativePath,
		Provider: "local",
	}, nil
}

func safeFileName(fileName string) string {
	baseName := filepath.Base(strings.TrimSpace(fileName))
	if baseName == "." || baseName == string(filepath.Separator) || baseName == "" {
		return "evidence"
	}

	var builder strings.Builder
	for _, char := range baseName {
		switch {
		case unicode.IsLetter(char), unicode.IsDigit(char):
			builder.WriteRune(char)
		case char == '.', char == '-', char == '_':
			builder.WriteRune(char)
		default:
			builder.WriteRune('_')
		}
	}

	cleaned := strings.Trim(builder.String(), "._-")
	if cleaned == "" {
		return "evidence"
	}

	return cleaned
}
