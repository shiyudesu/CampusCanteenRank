package cursor

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestEncodeDecode(t *testing.T) {
	now := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	encoded, err := Encode(Token{CreatedAt: now, ID: 12345})
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if decoded == nil {
		t.Fatalf("decoded token should not be nil")
	}
	if decoded.ID != 12345 {
		t.Fatalf("decoded id = %d, want 12345", decoded.ID)
	}
	if !decoded.CreatedAt.Equal(now) {
		t.Fatalf("decoded createdAt = %s, want %s", decoded.CreatedAt, now)
	}
}

func TestDecodeEmptyCursor(t *testing.T) {
	decoded, err := Decode("")
	if err != nil {
		t.Fatalf("decode empty failed: %v", err)
	}
	if decoded != nil {
		t.Fatalf("decode empty cursor should return nil token")
	}
}

func TestDecodeInvalidCursor(t *testing.T) {
	if _, err := Decode("invalid"); err == nil {
		t.Fatalf("decode invalid should return error")
	}
}

func TestEncodeDecodePreservesNanoseconds(t *testing.T) {
	now := time.Date(2026, 3, 22, 10, 0, 0, 123456789, time.UTC)
	encoded, err := Encode(Token{CreatedAt: now, ID: 67890})
	if err != nil {
		t.Fatalf("encode with nanos failed: %v", err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("decode with nanos failed: %v", err)
	}
	if decoded == nil {
		t.Fatalf("decoded token with nanos should not be nil")
	}
	if decoded.ID != 67890 {
		t.Fatalf("decoded id with nanos = %d, want 67890", decoded.ID)
	}
	if !decoded.CreatedAt.Equal(now) {
		t.Fatalf("decoded createdAt with nanos = %s, want %s", decoded.CreatedAt, now)
	}
}

func TestDecodeLegacyRFC3339Cursor(t *testing.T) {
	payload := map[string]interface{}{
		"createdAt": "2026-03-22T10:00:00Z",
		"id":        12345,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal legacy payload failed: %v", err)
	}
	legacyCursor := base64.RawURLEncoding.EncodeToString(raw)

	decoded, err := Decode(legacyCursor)
	if err != nil {
		t.Fatalf("decode legacy cursor failed: %v", err)
	}
	if decoded == nil {
		t.Fatalf("decoded legacy cursor should not be nil")
	}
	if decoded.ID != 12345 {
		t.Fatalf("decoded legacy id = %d, want 12345", decoded.ID)
	}
	wantTime := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	if !decoded.CreatedAt.Equal(wantTime) {
		t.Fatalf("decoded legacy createdAt = %s, want %s", decoded.CreatedAt, wantTime)
	}
}
