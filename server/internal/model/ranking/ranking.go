package model

import "time"

type RankingItem struct {
	StallID      int64
	StallName    string
	CanteenID    int64
	CanteenName  string
	FoodTypeID   int64
	FoodTypeName string
	AvgRating    float64
	RatingCount  int64
	ReviewCount  int64
	HotScore     float64
	LastActiveAt time.Time
}
