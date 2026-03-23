package dto

type CanteenItem struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Campus string `json:"campus"`
}

type CanteenListData struct {
	Items []CanteenItem `json:"items"`
}

type StallItem struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	CanteenID   int64   `json:"canteenId"`
	FoodTypeID  int64   `json:"foodTypeId"`
	AvgRating   float64 `json:"avgRating"`
	RatingCount int64   `json:"ratingCount"`
}

type StallListData struct {
	Items      []StallItem `json:"items"`
	NextCursor *string     `json:"nextCursor"`
	HasMore    bool        `json:"hasMore"`
}

type StallDetailData struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	CanteenID   int64   `json:"canteenId"`
	FoodTypeID  int64   `json:"foodTypeId"`
	AvgRating   float64 `json:"avgRating"`
	RatingCount int64   `json:"ratingCount"`
	MyRating    *int    `json:"myRating"`
}

type UpsertRatingRequest struct {
	Score int `json:"score" binding:"required,min=1,max=5"`
}

type UpsertRatingData struct {
	StallID     int64   `json:"stallId"`
	Score       int     `json:"score"`
	AvgRating   float64 `json:"avgRating"`
	RatingCount int64   `json:"ratingCount"`
}
