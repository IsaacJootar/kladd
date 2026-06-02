package audit

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeRecordScanner struct {
	values []any
}

func (scanner fakeRecordScanner) Scan(dest ...any) error {
	for index, value := range scanner.values {
		switch target := dest[index].(type) {
		case *uuid.UUID:
			*target = value.(uuid.UUID)
		case *string:
			*target = value.(string)
		case *[]byte:
			*target = value.([]byte)
		case *time.Time:
			*target = value.(time.Time)
		}
	}

	return nil
}

func TestScanRecordAcceptsMixedJSONMetadataValues(t *testing.T) {
	recordID := uuid.New()
	createdAt := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)

	record, err := scanRecord(fakeRecordScanner{
		values: []any{
			recordID,
			"security_pin.validation_succeeded",
			[]byte(`{"failed_attempts":0,"locked":false}`),
			createdAt,
		},
	})
	if err != nil {
		t.Fatalf("scan record: %v", err)
	}

	if record.ID != recordID {
		t.Fatalf("id = %s, want %s", record.ID, recordID)
	}
	if record.Metadata["failed_attempts"] != float64(0) {
		t.Fatalf("failed attempts = %#v, want 0", record.Metadata["failed_attempts"])
	}
	if record.Metadata["locked"] != false {
		t.Fatalf("locked = %#v, want false", record.Metadata["locked"])
	}
}
