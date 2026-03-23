package cursor

import (
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
