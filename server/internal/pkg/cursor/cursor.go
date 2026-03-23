package cursor

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

type Token struct {
	CreatedAt time.Time
	ID        int64
}

type tokenPayload struct {
	CreatedAt string `json:"createdAt"`
	ID        int64  `json:"id"`
}

func Encode(t Token) (string, error) {
	payload := tokenPayload{CreatedAt: t.CreatedAt.UTC().Format(time.RFC3339), ID: t.ID}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func Decode(encoded string) (*Token, error) {
	if encoded == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("invalid cursor")
	}
	var payload tokenPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, errors.New("invalid cursor")
	}
	if payload.ID <= 0 {
		return nil, errors.New("invalid cursor")
	}
	createdAt, err := time.Parse(time.RFC3339, payload.CreatedAt)
	if err != nil {
		return nil, errors.New("invalid cursor")
	}
	return &Token{CreatedAt: createdAt.UTC(), ID: payload.ID}, nil
}
