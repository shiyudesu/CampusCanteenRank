package dto

type CreateCommentRequest struct {
	Content       string `json:"content" binding:"required,min=1,max=2000"`
	RootID        int64  `json:"rootId"`
	ParentID      int64  `json:"parentId"`
	ReplyToUserID int64  `json:"replyToUserId"`
}

type CommentAuthorVO struct {
	ID       int64  `json:"id"`
	Nickname string `json:"nickname"`
}

type CommentItem struct {
	ID            int64           `json:"id"`
	StallID       int64           `json:"stallId"`
	RootID        int64           `json:"rootId"`
	ParentID      int64           `json:"parentId"`
	ReplyToUserID int64           `json:"replyToUserId"`
	Content       string          `json:"content"`
	LikeCount     int64           `json:"likeCount"`
	ReplyCount    int64           `json:"replyCount"`
	CreatedAt     string          `json:"createdAt"`
	Author        CommentAuthorVO `json:"author"`
	LikedByMe     bool            `json:"likedByMe"`
}

type CreateCommentData struct {
	Comment CommentItem `json:"comment"`
}

type CommentListData struct {
	Items      []CommentItem `json:"items"`
	NextCursor *string       `json:"nextCursor"`
	HasMore    bool          `json:"hasMore"`
}

type ToggleLikeData struct {
	Liked     bool  `json:"liked"`
	LikeCount int64 `json:"likeCount"`
}
