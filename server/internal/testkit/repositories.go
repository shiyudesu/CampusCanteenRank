package testkit

import (
	"context"
	"errors"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	authmodel "CampusCanteenRank/server/internal/model/auth"
	commentmodel "CampusCanteenRank/server/internal/model/comment"
	rankingmodel "CampusCanteenRank/server/internal/model/ranking"
	stallmodel "CampusCanteenRank/server/internal/model/stall"
	authrepo "CampusCanteenRank/server/internal/repository/auth"
	commentrepo "CampusCanteenRank/server/internal/repository/comment"
	rankingrepo "CampusCanteenRank/server/internal/repository/ranking"
	stallrepo "CampusCanteenRank/server/internal/repository/stall"
)

// UserRepository 是测试用内存用户仓储，仅供单元测试注入依赖。
type UserRepository struct {
	mu      sync.RWMutex
	nextID  int64
	byEmail map[string]*authmodel.User
	byID    map[int64]*authmodel.User
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		nextID:  1000,
		byEmail: make(map[string]*authmodel.User),
		byID:    make(map[int64]*authmodel.User),
	}
}

func (r *UserRepository) Create(_ context.Context, user *authmodel.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byEmail[user.Email]; exists {
		return authrepo.ErrAlreadyExists
	}
	r.nextID++
	clone := *user
	clone.ID = r.nextID
	clone.CreatedAt = time.Now().UTC()
	r.byEmail[user.Email] = &clone
	r.byID[clone.ID] = &clone
	user.ID = clone.ID
	user.CreatedAt = clone.CreatedAt
	return nil
}

func (r *UserRepository) GetByEmail(_ context.Context, email string) (*authmodel.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.byEmail[email]
	if !ok {
		return nil, authrepo.ErrNotFound
	}
	clone := *u
	return &clone, nil
}

func (r *UserRepository) GetByID(_ context.Context, id int64) (*authmodel.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.byID[id]
	if !ok {
		return nil, authrepo.ErrNotFound
	}
	clone := *u
	return &clone, nil
}

func (r *UserRepository) GetByIDs(_ context.Context, ids []int64) (map[int64]*authmodel.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[int64]*authmodel.User, len(ids))
	for _, id := range ids {
		u, ok := r.byID[id]
		if !ok {
			continue
		}
		clone := *u
		out[id] = &clone
	}
	return out, nil
}

// RefreshTokenRepository 是测试用内存 refresh-token 仓储。
type RefreshTokenRepository struct {
	mu     sync.Mutex
	active map[string]authrepo.RefreshTokenRecord
}

func NewRefreshTokenRepository() *RefreshTokenRepository {
	return &RefreshTokenRepository{active: make(map[string]authrepo.RefreshTokenRecord)}
}

func (r *RefreshTokenRepository) Save(_ context.Context, record authrepo.RefreshTokenRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := formatRecordKey(record.UserID, record.TokenJTI, record.DeviceID)
	r.active[key] = record
	return nil
}

func (r *RefreshTokenRepository) Consume(_ context.Context, userID int64, tokenJTI string, deviceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := formatRecordKey(userID, tokenJTI, deviceID)
	record, ok := r.active[key]
	if !ok {
		return authrepo.ErrNotFound
	}
	if time.Now().UTC().After(record.ExpiredAt) {
		delete(r.active, key)
		return authrepo.ErrNotFound
	}
	delete(r.active, key)
	return nil
}

func formatRecordKey(userID int64, tokenJTI string, deviceID string) string {
	return strconv.FormatInt(userID, 10) + ":" + deviceID + ":" + tokenJTI
}

// CommentRepository 是测试用内存评论仓储。
type CommentRepository struct {
	mu     sync.RWMutex
	nextID int64
	byID   map[int64]commentmodel.Comment
	likes  map[int64]map[int64]struct{}
}

func NewCommentRepository() *CommentRepository {
	now := time.Now().UTC()
	seed := map[int64]commentmodel.Comment{
		9001: {
			ID:            9001,
			StallID:       101,
			UserID:        1001,
			RootID:        0,
			ParentID:      0,
			ReplyToUserID: 0,
			Content:       "味道稳定，推荐",
			LikeCount:     12,
			ReplyCount:    3,
			Status:        1,
			CreatedAt:     now.Add(-3 * time.Minute),
		},
		9002: {
			ID:            9002,
			StallID:       101,
			UserID:        1002,
			RootID:        0,
			ParentID:      0,
			ReplyToUserID: 0,
			Content:       "出餐很快，午高峰排队也能接受",
			LikeCount:     5,
			ReplyCount:    1,
			Status:        1,
			CreatedAt:     now.Add(-2 * time.Minute),
		},
		9003: {
			ID:            9003,
			StallID:       101,
			UserID:        1003,
			RootID:        0,
			ParentID:      0,
			ReplyToUserID: 0,
			Content:       "今天红烧肉偏咸，下次希望稳定一点",
			LikeCount:     2,
			ReplyCount:    0,
			Status:        1,
			CreatedAt:     now.Add(-1 * time.Minute),
		},
	}
	return &CommentRepository{nextID: 9003, byID: seed, likes: make(map[int64]map[int64]struct{})}
}

func (r *CommentRepository) Create(_ context.Context, comment *commentmodel.Comment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	clone := *comment
	clone.ID = r.nextID
	clone.CreatedAt = time.Now().UTC()
	clone.Status = 1
	r.byID[clone.ID] = clone
	comment.ID = clone.ID
	comment.CreatedAt = clone.CreatedAt
	comment.Status = clone.Status
	return nil
}

func (r *CommentRepository) CreateReplyAndIncrementRoot(_ context.Context, reply *commentmodel.Comment, rootID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	root, ok := r.byID[rootID]
	if !ok || root.Status != 1 || root.RootID != 0 || root.ParentID != 0 {
		return commentrepo.ErrNotFound
	}
	r.nextID++
	clone := *reply
	clone.ID = r.nextID
	clone.CreatedAt = time.Now().UTC()
	clone.Status = 1
	r.byID[clone.ID] = clone
	root.ReplyCount++
	r.byID[rootID] = root
	reply.ID = clone.ID
	reply.CreatedAt = clone.CreatedAt
	reply.Status = clone.Status
	return nil
}

func (r *CommentRepository) GetByID(_ context.Context, commentID int64) (*commentmodel.Comment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.byID[commentID]
	if !ok || item.Status != 1 {
		return nil, commentrepo.ErrNotFound
	}
	clone := item
	return &clone, nil
}

func (r *CommentRepository) IncrementRootReplyCount(_ context.Context, rootID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.byID[rootID]
	if !ok || item.Status != 1 || item.RootID != 0 || item.ParentID != 0 {
		return commentrepo.ErrNotFound
	}
	item.ReplyCount++
	r.byID[rootID] = item
	return nil
}

func (r *CommentRepository) Like(_ context.Context, userID int64, commentID int64) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.byID[commentID]
	if !ok || item.Status != 1 {
		return 0, commentrepo.ErrNotFound
	}
	if _, ok := r.likes[commentID]; !ok {
		r.likes[commentID] = make(map[int64]struct{})
	}
	if _, exists := r.likes[commentID][userID]; exists {
		return item.LikeCount, nil
	}
	r.likes[commentID][userID] = struct{}{}
	item.LikeCount++
	r.byID[commentID] = item
	return item.LikeCount, nil
}

func (r *CommentRepository) Unlike(_ context.Context, userID int64, commentID int64) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.byID[commentID]
	if !ok || item.Status != 1 {
		return 0, commentrepo.ErrNotFound
	}
	userSet, ok := r.likes[commentID]
	if !ok {
		return item.LikeCount, nil
	}
	if _, exists := userSet[userID]; !exists {
		return item.LikeCount, nil
	}
	delete(userSet, userID)
	if item.LikeCount > 0 {
		item.LikeCount--
	}
	r.byID[commentID] = item
	return item.LikeCount, nil
}

func (r *CommentRepository) HasLiked(_ context.Context, userID int64, commentID int64) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.byID[commentID]
	if !ok || item.Status != 1 {
		return false, commentrepo.ErrNotFound
	}
	userSet, ok := r.likes[commentID]
	if !ok {
		return false, nil
	}
	_, exists := userSet[userID]
	return exists, nil
}

func (r *CommentRepository) HasLikedBatch(_ context.Context, userID int64, commentIDs []int64) (map[int64]bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[int64]bool, len(commentIDs))
	for _, commentID := range commentIDs {
		item, ok := r.byID[commentID]
		if !ok || item.Status != 1 {
			result[commentID] = false
			continue
		}
		userSet, ok := r.likes[commentID]
		if !ok {
			result[commentID] = false
			continue
		}
		_, liked := userSet[userID]
		result[commentID] = liked
	}
	return result, nil
}

func (r *CommentRepository) ListTopLevelByStall(_ context.Context, options commentrepo.CommentListOptions) ([]commentmodel.Comment, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]commentmodel.Comment, 0, len(r.byID))
	for _, item := range r.byID {
		if item.Status != 1 || item.StallID != options.StallID || item.RootID != 0 || item.ParentID != 0 {
			continue
		}
		list = append(list, item)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].ID > list[j].ID
		}
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	filtered := make([]commentmodel.Comment, 0, len(list))
	for _, item := range list {
		if options.Cursor != nil {
			if item.CreatedAt.After(options.Cursor.CreatedAt) {
				continue
			}
			if item.CreatedAt.Equal(options.Cursor.CreatedAt) && item.ID >= options.Cursor.ID {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 20
	}
	hasMore := len(filtered) > limit
	if hasMore {
		filtered = filtered[:limit]
	}
	out := make([]commentmodel.Comment, len(filtered))
	copy(out, filtered)
	return out, hasMore, nil
}

func (r *CommentRepository) ListRepliesByRoot(_ context.Context, rootCommentID int64, limit int, cursor *commentrepo.CommentCursor) ([]commentmodel.Comment, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]commentmodel.Comment, 0, len(r.byID))
	for _, item := range r.byID {
		if item.Status != 1 || item.RootID != rootCommentID || item.ParentID == 0 {
			continue
		}
		list = append(list, item)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].ID > list[j].ID
		}
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	filtered := make([]commentmodel.Comment, 0, len(list))
	for _, item := range list {
		if cursor != nil {
			if item.CreatedAt.After(cursor.CreatedAt) {
				continue
			}
			if item.CreatedAt.Equal(cursor.CreatedAt) && item.ID >= cursor.ID {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	if limit <= 0 {
		limit = 20
	}
	hasMore := len(filtered) > limit
	if hasMore {
		filtered = filtered[:limit]
	}
	out := make([]commentmodel.Comment, len(filtered))
	copy(out, filtered)
	return out, hasMore, nil
}

func (r *CommentRepository) ListByUser(_ context.Context, userID int64, limit int, cursor *commentrepo.CommentCursor) ([]commentmodel.Comment, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]commentmodel.Comment, 0, len(r.byID))
	for _, item := range r.byID {
		if item.Status != 1 || item.UserID != userID {
			continue
		}
		list = append(list, item)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].ID > list[j].ID
		}
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	filtered := make([]commentmodel.Comment, 0, len(list))
	for _, item := range list {
		if cursor != nil {
			if item.CreatedAt.After(cursor.CreatedAt) {
				continue
			}
			if item.CreatedAt.Equal(cursor.CreatedAt) && item.ID >= cursor.ID {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	if limit <= 0 {
		limit = 20
	}
	hasMore := len(filtered) > limit
	if hasMore {
		filtered = filtered[:limit]
	}
	out := make([]commentmodel.Comment, len(filtered))
	copy(out, filtered)
	return out, hasMore, nil
}

// StallRepository 是测试用内存摊位仓储。
type StallRepository struct {
	mu       sync.RWMutex
	canteens []stallmodel.Canteen
	stalls   []stallmodel.Stall
	ratings  map[int64]map[int64]stallmodel.UserRating
}

func NewStallRepository() *StallRepository {
	now := time.Now().UTC()
	stalls := []stallmodel.Stall{
		{ID: 101, CanteenID: 1, FoodTypeID: 2, Name: "川味小炒", AvgRating: 4.6, RatingCount: 532, Status: 1, CreatedAt: now.Add(-1 * time.Minute)},
		{ID: 102, CanteenID: 1, FoodTypeID: 3, Name: "北方面食", AvgRating: 4.2, RatingCount: 321, Status: 1, CreatedAt: now.Add(-2 * time.Minute)},
		{ID: 201, CanteenID: 2, FoodTypeID: 1, Name: "轻食沙拉", AvgRating: 4.8, RatingCount: 188, Status: 1, CreatedAt: now.Add(-3 * time.Minute)},
	}
	sort.Slice(stalls, func(i, j int) bool {
		if stalls[i].CreatedAt.Equal(stalls[j].CreatedAt) {
			return stalls[i].ID > stalls[j].ID
		}
		return stalls[i].CreatedAt.After(stalls[j].CreatedAt)
	})
	return &StallRepository{
		canteens: []stallmodel.Canteen{{ID: 1, Name: "一食堂", Campus: "主校区", Status: 1}, {ID: 2, Name: "二食堂", Campus: "东校区", Status: 1}},
		stalls:   stalls,
		ratings:  map[int64]map[int64]stallmodel.UserRating{},
	}
}

func (r *StallRepository) ListCanteens(_ context.Context) ([]stallmodel.Canteen, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]stallmodel.Canteen, 0, len(r.canteens))
	for _, c := range r.canteens {
		if c.Status == 1 {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *StallRepository) ListStalls(_ context.Context, options stallrepo.StallListOptions) ([]stallmodel.Stall, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	base := make([]stallmodel.Stall, 0, len(r.stalls))
	for _, s := range r.stalls {
		if s.Status != 1 {
			continue
		}
		if options.Filter.CanteenID > 0 && s.CanteenID != options.Filter.CanteenID {
			continue
		}
		if options.Filter.FoodTypeID > 0 && s.FoodTypeID != options.Filter.FoodTypeID {
			continue
		}
		base = append(base, s)
	}
	sort.Slice(base, func(i, j int) bool {
		left := int64(math.Round(base[i].AvgRating * 100))
		right := int64(math.Round(base[j].AvgRating * 100))
		if left == right {
			return base[i].ID > base[j].ID
		}
		return left > right
	})
	filtered := make([]stallmodel.Stall, 0, len(base))
	for _, s := range base {
		if options.Cursor != nil {
			ratingKey := int64(math.Round(s.AvgRating * 100))
			if ratingKey > options.Cursor.AvgRatingX100 {
				continue
			}
			if ratingKey == options.Cursor.AvgRatingX100 && s.ID >= options.Cursor.ID {
				continue
			}
		}
		filtered = append(filtered, s)
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 20
	}
	hasMore := len(filtered) > limit
	if hasMore {
		filtered = filtered[:limit]
	}
	out := make([]stallmodel.Stall, len(filtered))
	copy(out, filtered)
	return out, hasMore, nil
}

func (r *StallRepository) GetStallByID(_ context.Context, stallID int64) (*stallmodel.Stall, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, s := range r.stalls {
		if s.ID == stallID && s.Status == 1 {
			clone := s
			return &clone, nil
		}
	}
	return nil, stallrepo.ErrNotFound
}

func (r *StallRepository) GetStallsByIDs(_ context.Context, stallIDs []int64) (map[int64]*stallmodel.Stall, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	set := make(map[int64]struct{}, len(stallIDs))
	for _, id := range stallIDs {
		set[id] = struct{}{}
	}
	out := make(map[int64]*stallmodel.Stall, len(stallIDs))
	for _, stall := range r.stalls {
		if _, ok := set[stall.ID]; !ok || stall.Status != 1 {
			continue
		}
		clone := stall
		out[stall.ID] = &clone
	}
	return out, nil
}

func (r *StallRepository) GetUserRating(_ context.Context, userID int64, stallID int64) (*int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	byStall, ok := r.ratings[userID]
	if !ok {
		return nil, nil
	}
	rating, ok := byStall[stallID]
	if !ok {
		return nil, nil
	}
	result := rating.Score
	return &result, nil
}

func (r *StallRepository) UpsertUserRating(_ context.Context, userID int64, stallID int64, score int) (*stallmodel.Stall, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	stallIndex := -1
	for i, s := range r.stalls {
		if s.ID == stallID && s.Status == 1 {
			stallIndex = i
			break
		}
	}
	if stallIndex < 0 {
		return nil, stallrepo.ErrNotFound
	}
	if _, ok := r.ratings[userID]; !ok {
		r.ratings[userID] = make(map[int64]stallmodel.UserRating)
	}
	previous, hadPrevious := r.ratings[userID][stallID]
	r.ratings[userID][stallID] = stallmodel.UserRating{UserID: userID, StallID: stallID, Score: score, UpdatedAt: time.Now().UTC()}
	target := &r.stalls[stallIndex]
	if hadPrevious {
		if target.RatingCount <= 0 {
			return nil, errors.New("invalid aggregate state")
		}
		total := target.AvgRating*float64(target.RatingCount) - float64(previous.Score) + float64(score)
		target.AvgRating = total / float64(target.RatingCount)
	} else {
		total := target.AvgRating*float64(target.RatingCount) + float64(score)
		target.RatingCount++
		target.AvgRating = total / float64(target.RatingCount)
	}
	clone := *target
	return &clone, nil
}

func (r *StallRepository) ListUserRatings(_ context.Context, userID int64, limit int, cursor *stallrepo.UserRatingCursor) ([]stallmodel.UserRating, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 {
		limit = 20
	}
	byStall, ok := r.ratings[userID]
	if !ok {
		return []stallmodel.UserRating{}, false, nil
	}
	items := make([]stallmodel.UserRating, 0, len(byStall))
	for _, item := range byStall {
		if cursor != nil {
			if item.UpdatedAt.After(cursor.UpdatedAt) {
				continue
			}
			if item.UpdatedAt.Equal(cursor.UpdatedAt) && item.StallID >= cursor.StallID {
				continue
			}
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].StallID > items[j].StallID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	out := make([]stallmodel.UserRating, len(items))
	copy(out, items)
	return out, hasMore, nil
}

// RankingRepository 是测试用内存排行榜仓储。
type RankingRepository struct {
	items []rankingmodel.RankingItem
}

func NewRankingRepository() *RankingRepository {
	now := time.Now().UTC()
	return &RankingRepository{items: []rankingmodel.RankingItem{
		{StallID: 101, StallName: "川味小炒", CanteenID: 1, CanteenName: "一食堂", FoodTypeID: 2, FoodTypeName: "川菜", AvgRating: 4.61, RatingCount: 532, ReviewCount: 221, HotScore: 97.4, LastActiveAt: now.Add(-10 * time.Minute)},
		{StallID: 201, StallName: "轻食沙拉", CanteenID: 2, CanteenName: "二食堂", FoodTypeID: 1, FoodTypeName: "轻食", AvgRating: 4.80, RatingCount: 188, ReviewCount: 95, HotScore: 93.1, LastActiveAt: now.Add(-15 * time.Minute)},
		{StallID: 102, StallName: "北方面食", CanteenID: 1, CanteenName: "一食堂", FoodTypeID: 3, FoodTypeName: "面食", AvgRating: 4.20, RatingCount: 321, ReviewCount: 160, HotScore: 90.2, LastActiveAt: now.Add(-20 * time.Minute)},
		{StallID: 301, StallName: "麻辣香锅", CanteenID: 3, CanteenName: "三食堂", FoodTypeID: 2, FoodTypeName: "川菜", AvgRating: 4.35, RatingCount: 280, ReviewCount: 143, HotScore: 91.8, LastActiveAt: now.Add(-25 * time.Minute)},
		{StallID: 302, StallName: "石锅拌饭", CanteenID: 3, CanteenName: "三食堂", FoodTypeID: 4, FoodTypeName: "韩餐", AvgRating: 4.05, RatingCount: 170, ReviewCount: 88, HotScore: 84.6, LastActiveAt: now.Add(-40 * time.Minute)},
	}}
}

func (r *RankingRepository) ListRankings(_ context.Context, options rankingrepo.RankingListOptions) ([]rankingmodel.RankingItem, bool, error) {
	if options.Filter.Scope != "global" && options.Filter.Scope != "canteen" && options.Filter.Scope != "foodType" {
		return nil, false, rankingrepo.ErrInvalidScope
	}
	base := make([]rankingmodel.RankingItem, 0, len(r.items))
	for _, item := range r.items {
		if options.Filter.Scope == "canteen" && item.CanteenID != options.Filter.ScopeID {
			continue
		}
		if options.Filter.Scope == "foodType" && item.FoodTypeID != options.Filter.ScopeID {
			continue
		}
		if options.Filter.FoodTypeID > 0 && item.FoodTypeID != options.Filter.FoodTypeID {
			continue
		}
		base = append(base, item)
	}
	if options.Filter.Sort == "hot_desc" {
		sort.Slice(base, func(i, j int) bool {
			if base[i].HotScore == base[j].HotScore {
				if base[i].LastActiveAt.Equal(base[j].LastActiveAt) {
					return base[i].StallID > base[j].StallID
				}
				return base[i].LastActiveAt.After(base[j].LastActiveAt)
			}
			return base[i].HotScore > base[j].HotScore
		})
	} else {
		sort.Slice(base, func(i, j int) bool {
			if base[i].AvgRating == base[j].AvgRating {
				if base[i].LastActiveAt.Equal(base[j].LastActiveAt) {
					return base[i].StallID > base[j].StallID
				}
				return base[i].LastActiveAt.After(base[j].LastActiveAt)
			}
			return base[i].AvgRating > base[j].AvgRating
		})
	}

	// 游标过滤遵循稳定排序规则，避免翻页重复/漏数。
	filtered := make([]rankingmodel.RankingItem, 0, len(base))
	for _, item := range base {
		if options.Cursor == nil {
			filtered = append(filtered, item)
			continue
		}
		value := item.AvgRating
		if options.Filter.Sort == "hot_desc" {
			value = item.HotScore
		}
		if value > options.Cursor.SortValue {
			continue
		}
		if value == options.Cursor.SortValue {
			if item.LastActiveAt.After(options.Cursor.LastActiveAt) {
				continue
			}
			if item.LastActiveAt.Equal(options.Cursor.LastActiveAt) && item.StallID >= options.Cursor.StallID {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 20
	}
	hasMore := len(filtered) > limit
	if hasMore {
		filtered = filtered[:limit]
	}
	out := make([]rankingmodel.RankingItem, len(filtered))
	copy(out, filtered)
	return out, hasMore, nil
}
