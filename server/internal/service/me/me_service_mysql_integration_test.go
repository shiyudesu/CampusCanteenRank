package service

import (
	"context"
	"os"
	"testing"

	"CampusCanteenRank/server/internal/migration"
	commentmodel "CampusCanteenRank/server/internal/model/comment"
	authrepo "CampusCanteenRank/server/internal/repository/auth"
	commentrepo "CampusCanteenRank/server/internal/repository/comment"
	stallrepo "CampusCanteenRank/server/internal/repository/stall"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func openMeMySQLTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		t.Skip("MYSQL_DSN is empty, skip mysql integration test")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("mysql unavailable for integration test: %v", err)
	}
	if err := migration.ApplySQLMigrations(db); err != nil {
		t.Skipf("apply sql migrations failed for integration test: %v", err)
	}
	return db
}

func TestMySQLMeServiceListMyCommentsReturnsLikedByMeState(t *testing.T) {
	db := openMeMySQLTestDB(t)
	ctx := context.Background()

	for _, sql := range []string{
		"DELETE FROM comment_likes",
		"DELETE FROM comments",
		"DELETE FROM ratings",
		"DELETE FROM stalls",
		"DELETE FROM canteens",
		"DELETE FROM users",
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	}

	if err := db.Exec("INSERT INTO users (id, nickname, email, password_hash, status) VALUES (1001, 'Tom', 'tom-me@example.com', 'hashed', 1)").Error; err != nil {
		t.Fatalf("seed users failed: %v", err)
	}
	if err := db.Exec("INSERT INTO canteens (id, name, campus, status) VALUES (1, '一食堂', '主校区', 1)").Error; err != nil {
		t.Fatalf("seed canteen failed: %v", err)
	}
	if err := db.Exec("INSERT INTO stalls (id, canteen_id, food_type_id, name, avg_rating, rating_count, status) VALUES (101, 1, 2, '川味小炒', 0, 0, 1)").Error; err != nil {
		t.Fatalf("seed stall failed: %v", err)
	}

	commentRepository, err := commentrepo.NewMySQLCommentRepository(db)
	if err != nil {
		t.Fatalf("new mysql comment repository failed: %v", err)
	}
	seed := &commentmodel.Comment{StallID: 101, UserID: 1001, RootID: 0, ParentID: 0, ReplyToUserID: 0, Content: "my top-level", LikeCount: 0, ReplyCount: 0, Status: 1}
	if err := commentRepository.Create(ctx, seed); err != nil {
		t.Fatalf("seed comment create failed: %v", err)
	}
	if _, likeErr := commentRepository.Like(ctx, 1001, seed.ID); likeErr != nil {
		t.Fatalf("seed like failed: %v", likeErr)
	}

	userRepository, err := authrepo.NewMySQLUserRepository(db)
	if err != nil {
		t.Fatalf("new mysql user repository failed: %v", err)
	}
	stallRepository, err := stallrepo.NewMySQLStallRepository(db)
	if err != nil {
		t.Fatalf("new mysql stall repository failed: %v", err)
	}
	service := NewMeService(commentRepository, stallRepository, userRepository)

	data, err := service.ListMyComments(ctx, 1001, 20, "")
	if err != nil {
		t.Fatalf("list my comments failed: %v", err)
	}
	if len(data.Items) == 0 {
		t.Fatalf("my comments should not be empty")
	}
	if !data.Items[0].LikedByMe {
		t.Fatalf("likedByMe = false, want true")
	}
	if data.Items[0].Author.Nickname != "Tom" {
		t.Fatalf("author nickname = %q, want %q", data.Items[0].Author.Nickname, "Tom")
	}
}

func TestMySQLMeServiceListMyRatingsPagination(t *testing.T) {
	db := openMeMySQLTestDB(t)
	ctx := context.Background()

	for _, sql := range []string{
		"DELETE FROM ratings",
		"DELETE FROM stalls",
		"DELETE FROM canteens",
		"DELETE FROM users",
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	}

	if err := db.Exec("INSERT INTO users (id, nickname, email, password_hash, status) VALUES (1001, 'Tom', 'tom-rating@example.com', 'hashed', 1)").Error; err != nil {
		t.Fatalf("seed users failed: %v", err)
	}
	if err := db.Exec("INSERT INTO canteens (id, name, campus, status) VALUES (1, '一食堂', '主校区', 1)").Error; err != nil {
		t.Fatalf("seed canteen failed: %v", err)
	}
	if err := db.Exec("INSERT INTO stalls (id, canteen_id, food_type_id, name, avg_rating, rating_count, status) VALUES (101, 1, 2, '川味小炒', 0, 0, 1), (102, 1, 2, '北方面档', 0, 0, 1), (103, 1, 2, '粤式烧腊', 0, 0, 1)").Error; err != nil {
		t.Fatalf("seed stalls failed: %v", err)
	}

	userRepository, err := authrepo.NewMySQLUserRepository(db)
	if err != nil {
		t.Fatalf("new mysql user repository failed: %v", err)
	}
	commentRepository, err := commentrepo.NewMySQLCommentRepository(db)
	if err != nil {
		t.Fatalf("new mysql comment repository failed: %v", err)
	}
	stallRepository, err := stallrepo.NewMySQLStallRepository(db)
	if err != nil {
		t.Fatalf("new mysql stall repository failed: %v", err)
	}

	for _, payload := range []struct {
		stallID int64
		score   int
	}{{stallID: 101, score: 5}, {stallID: 102, score: 4}, {stallID: 103, score: 3}} {
		if _, upsertErr := stallRepository.UpsertUserRating(ctx, 1001, payload.stallID, payload.score); upsertErr != nil {
			t.Fatalf("upsert user rating failed for stall %d: %v", payload.stallID, upsertErr)
		}
	}

	service := NewMeService(commentRepository, stallRepository, userRepository)
	first, err := service.ListMyRatings(ctx, 1001, 2, "")
	if err != nil {
		t.Fatalf("list my ratings first page failed: %v", err)
	}
	if len(first.Items) != 2 {
		t.Fatalf("first page len = %d, want 2", len(first.Items))
	}
	if !first.HasMore {
		t.Fatalf("first page hasMore = false, want true")
	}
	if first.NextCursor == nil || *first.NextCursor == "" {
		t.Fatalf("first page nextCursor should not be empty")
	}

	second, err := service.ListMyRatings(ctx, 1001, 2, *first.NextCursor)
	if err != nil {
		t.Fatalf("list my ratings second page failed: %v", err)
	}
	if len(second.Items) != 1 {
		t.Fatalf("second page len = %d, want 1", len(second.Items))
	}
	if second.HasMore {
		t.Fatalf("second page hasMore = true, want false")
	}
}
