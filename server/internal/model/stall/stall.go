package model

import "time"

type Canteen struct {
	ID     int64
	Name   string
	Campus string
	Status int8
}

type Stall struct {
	ID          int64
	CanteenID   int64
	FoodTypeID  int64
	Name        string
	AvgRating   float64
	RatingCount int64
	Status      int8
	CreatedAt   time.Time
}
