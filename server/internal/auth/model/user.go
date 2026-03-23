package model

import "time"

type User struct {
	ID           int64
	Nickname     string
	Email        string
	PasswordHash string
	Status       int
	CreatedAt    time.Time
}
