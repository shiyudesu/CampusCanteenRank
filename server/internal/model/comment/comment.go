package model

import "time"

type Comment struct {
	ID            int64
	StallID       int64
	UserID        int64
	RootID        int64
	ParentID      int64
	ReplyToUserID int64
	Content       string
	LikeCount     int64
	ReplyCount    int64
	Status        int8
	CreatedAt     time.Time
}
