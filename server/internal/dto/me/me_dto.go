package dto

import (
	commentdto "CampusCanteenRank/server/internal/dto/comment"
	stalldto "CampusCanteenRank/server/internal/dto/stall"
)

type MyCommentListData struct {
	Items      []commentdto.CommentItem `json:"items"`
	NextCursor *string                  `json:"nextCursor"`
	HasMore    bool                     `json:"hasMore"`
}

type MyRatingItem struct {
	StallID   int64  `json:"stallId"`
	StallName string `json:"stallName"`
	Score     int    `json:"score"`
	UpdatedAt string `json:"updatedAt"`
}

type MyRatingListData struct {
	Items      []MyRatingItem `json:"items"`
	NextCursor *string        `json:"nextCursor"`
	HasMore    bool           `json:"hasMore"`
}

func ToUpsertRatingData(item MyRatingItem, avgRating float64, ratingCount int64) stalldto.UpsertRatingData {
	return stalldto.UpsertRatingData{
		StallID:     item.StallID,
		Score:       item.Score,
		AvgRating:   avgRating,
		RatingCount: ratingCount,
	}
}
