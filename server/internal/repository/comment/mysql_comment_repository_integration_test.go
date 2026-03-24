package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	model "CampusCanteenRank/server/internal/model/comment"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func openCommentMySQLTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		t.Skip("MYSQL_DSN is empty, skip mysql integration test")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("mysql unavailable for integration test: %v", err)
	}
	return db
}

func newCommentMySQLRepoForTest(t *testing.T) *MySQLCommentRepository {
	t.Helper()
	db := openCommentMySQLTestDB(t)
	repo, err := NewMySQLCommentRepository(db)
	if err != nil {
		t.Fatalf("new mysql comment repository failed: %v", err)
	}
	if err := db.Exec("DELETE FROM comment_likes").Error; err != nil {
		t.Fatalf("cleanup comment_likes failed: %v", err)
	}
	if err := db.Exec("DELETE FROM comments").Error; err != nil {
		t.Fatalf("cleanup comments failed: %v", err)
	}
	return repo
}

func TestMySQLCommentRepositoryLikeUnlikeIdempotent(t *testing.T) {
	repo := newCommentMySQLRepoForTest(t)
	ctx := context.Background()

	seed := &model.Comment{StallID: 101, UserID: 1001, RootID: 0, ParentID: 0, ReplyToUserID: 0, Content: "mysql like", LikeCount: 0, ReplyCount: 0, Status: 1}
	if err := repo.Create(ctx, seed); err != nil {
		t.Fatalf("create seed comment failed: %v", err)
	}

	count, err := repo.Like(ctx, 2001, seed.ID)
	if err != nil {
		t.Fatalf("first like failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("first like count = %d, want 1", count)
	}

	count, err = repo.Like(ctx, 2001, seed.ID)
	if err != nil {
		t.Fatalf("duplicate like should be idempotent: %v", err)
	}
	if count != 1 {
		t.Fatalf("duplicate like count = %d, want 1", count)
	}

	count, err = repo.Unlike(ctx, 2001, seed.ID)
	if err != nil {
		t.Fatalf("first unlike failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("first unlike count = %d, want 0", count)
	}

	count, err = repo.Unlike(ctx, 2001, seed.ID)
	if err != nil {
		t.Fatalf("duplicate unlike should be idempotent: %v", err)
	}
	if count != 0 {
		t.Fatalf("duplicate unlike count = %d, want 0", count)
	}
}

func TestMySQLCommentRepositoryHasLikedBatchEdgeCases(t *testing.T) {
	repo := newCommentMySQLRepoForTest(t)
	ctx := context.Background()

	a := &model.Comment{StallID: 101, UserID: 1001, RootID: 0, ParentID: 0, ReplyToUserID: 0, Content: "A", Status: 1}
	b := &model.Comment{StallID: 101, UserID: 1002, RootID: 0, ParentID: 0, ReplyToUserID: 0, Content: "B", Status: 1}
	if err := repo.Create(ctx, a); err != nil {
		t.Fatalf("create comment A failed: %v", err)
	}
	if err := repo.Create(ctx, b); err != nil {
		t.Fatalf("create comment B failed: %v", err)
	}

	if _, err := repo.Like(ctx, 3001, a.ID); err != nil {
		t.Fatalf("seed like failed: %v", err)
	}

	liked, err := repo.HasLikedBatch(ctx, 3001, []int64{a.ID, b.ID, 999999, -1, 0})
	if err != nil {
		t.Fatalf("has liked batch failed: %v", err)
	}
	if !liked[a.ID] {
		t.Fatalf("comment A should be liked")
	}
	if liked[b.ID] {
		t.Fatalf("comment B should not be liked")
	}
	if liked[999999] {
		t.Fatalf("unknown comment should be false")
	}
	if _, exists := liked[-1]; exists {
		t.Fatalf("negative comment id should not be included in result map")
	}
	if _, exists := liked[0]; exists {
		t.Fatalf("zero comment id should not be included in result map")
	}

	empty, err := repo.HasLikedBatch(ctx, 3001, nil)
	if err != nil {
		t.Fatalf("empty has liked batch failed: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("empty has liked batch result len = %d, want 0", len(empty))
	}
}

func TestMySQLCommentRepositoryLikeUnlikeNotFound(t *testing.T) {
	repo := newCommentMySQLRepoForTest(t)
	ctx := context.Background()

	if _, err := repo.Like(ctx, 1001, 999999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("like unknown comment error = %v, want ErrNotFound", err)
	}
	if _, err := repo.Unlike(ctx, 1001, 999999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unlike unknown comment error = %v, want ErrNotFound", err)
	}
	if _, err := repo.HasLiked(ctx, 1001, 999999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("hasLiked unknown comment error = %v, want ErrNotFound", err)
	}
}

func TestMySQLCommentRepositoryListTopLevelCursorStability(t *testing.T) {
	repo := newCommentMySQLRepoForTest(t)
	ctx := context.Background()

	createdIDs := make([]int64, 0, 3)
	for i := range 3 {
		c := &model.Comment{StallID: 101, UserID: int64(1100 + i), RootID: 0, ParentID: 0, ReplyToUserID: 0, Content: "same-ts", Status: 1}
		if err := repo.Create(ctx, c); err != nil {
			t.Fatalf("create top-level comment[%d] failed: %v", i, err)
		}
		createdIDs = append(createdIDs, c.ID)
	}

	baseTime := time.Now().UTC().Truncate(time.Second)
	if err := repo.db.WithContext(ctx).Exec(
		"UPDATE comments SET created_at = ? WHERE id IN (?, ?, ?)",
		baseTime,
		createdIDs[0],
		createdIDs[1],
		createdIDs[2],
	).Error; err != nil {
		t.Fatalf("align created_at for cursor stability test failed: %v", err)
	}

	seen := make(map[int64]struct{}, len(createdIDs))
	var cursor *CommentCursor
	for page := range 5 {
		items, hasMore, err := repo.ListTopLevelByStall(ctx, CommentListOptions{StallID: 101, Limit: 1, Cursor: cursor})
		if err != nil {
			t.Fatalf("list page[%d] failed: %v", page, err)
		}
		if len(items) == 0 {
			break
		}
		id := items[0].ID
		if _, exists := seen[id]; exists {
			t.Fatalf("pagination duplicated comment id=%d", id)
		}
		seen[id] = struct{}{}
		cursor = &CommentCursor{CreatedAt: items[0].CreatedAt, ID: items[0].ID}
		if !hasMore {
			break
		}
	}

	for _, id := range createdIDs {
		if _, exists := seen[id]; !exists {
			t.Fatalf("cursor pagination missed created id=%d", id)
		}
	}
}
