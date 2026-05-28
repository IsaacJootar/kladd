package database

import (
	"context"
	"errors"
	"testing"
)

func TestOpenRequiresDatabaseURL(t *testing.T) {
	db, err := Open(context.Background(), "")
	if db != nil {
		t.Fatal("expected nil database handle")
	}

	if !errors.Is(err, ErrDatabaseURLRequired) {
		t.Fatalf("error = %v, want %v", err, ErrDatabaseURLRequired)
	}
}
