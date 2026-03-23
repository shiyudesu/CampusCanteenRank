package dto

type RankingItem struct {
	Rank         int     `json:"rank"`
	StallID      int64   `json:"stallId"`
	StallName    string  `json:"stallName"`
	CanteenID    int64   `json:"canteenId"`
	CanteenName  string  `json:"canteenName"`
	FoodTypeID   int64   `json:"foodTypeId"`
	FoodTypeName string  `json:"foodTypeName"`
	AvgRating    float64 `json:"avgRating"`
	RatingCount  int64   `json:"ratingCount"`
	ReviewCount  int64   `json:"reviewCount"`
	HotScore     float64 `json:"hotScore"`
}

type RankingListData struct {
	Items      []RankingItem `json:"items"`
	NextCursor *string       `json:"nextCursor"`
	HasMore    bool          `json:"hasMore"`
}
