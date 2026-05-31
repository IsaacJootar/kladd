package evidence

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestLocalStorageSavesInsideEvidenceDirectory(t *testing.T) {
	root := t.TempDir()
	storage := NewLocalStorage(root)
	userID := uuid.New()
	evidenceID := uuid.New()

	stored, err := storage.Save(context.Background(), userID, evidenceID, "../passport copy.pdf", bytes.NewBufferString("fake-content"))
	if err != nil {
		t.Fatalf("save evidence: %v", err)
	}

	if filepath.IsAbs(stored.Path) {
		t.Fatalf("stored path = %q, want relative path", stored.Path)
	}

	if stored.Provider != "local" {
		t.Fatalf("provider = %q, want local", stored.Provider)
	}

	fullPath := filepath.Join(root, stored.Path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}

	if string(content) != "fake-content" {
		t.Fatalf("content = %q, want fake-content", string(content))
	}
}

func TestSafeFileName(t *testing.T) {
	tests := map[string]string{
		"../passport copy.pdf": "passport_copy.pdf",
		"nin#scan?.png":        "nin_scan_.png",
		"   ":                  "evidence",
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			if got := safeFileName(input); got != want {
				t.Fatalf("safe file name = %q, want %q", got, want)
			}
		})
	}
}
