package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func openRankingMySQLTestDB(t *testing.T) *gorm.DB {
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

func newRankingMySQLRepoForTest(t *testing.T) *MySQLRankingRepository {
	t.Helper()
	db := openRankingMySQLTestDB(t)
	repo, err := NewMySQLRankingRepository(db)
	if err != nil {
		t.Fatalf("new mysql ranking repository failed: %v", err)
	}
	if err := db.Exec("DELETE FROM ratings").Error; err != nil {
		t.Fatalf("cleanup ratings failed: %v", err)
	}
	if err := db.Exec("DELETE FROM comments").Error; err != nil {
		t.Fatalf("cleanup comments failed: %v", err)
	}
	if err := db.Exec("DELETE FROM stalls").Error; err != nil {
		t.Fatalf("cleanup stalls failed: %v", err)
	}
	if err := db.Exec("DELETE FROM canteens").Error; err != nil {
		t.Fatalf("cleanup canteens failed: %v", err)
	}
	if err := db.Exec("DELETE FROM food_types").Error; err != nil {
		t.Fatalf("cleanup food_types failed: %v", err)
	}

	if err := db.Exec("INSERT INTO canteens (id, name, campus, status) VALUES (1, '一食堂', '主校区', 1)").Error; err != nil {
		t.Fatalf("seed canteen failed: %v", err)
	}
	if err := db.Exec("INSERT INTO food_types (id, name) VALUES (2, '川菜')").Error; err != nil {
		t.Fatalf("seed food type failed: %v", err)
	}

	now := time.Now().UTC()
	if err := db.Exec("INSERT INTO stalls (id, canteen_id, food_type_id, name, avg_rating, rating_count, status, created_at) VALUES (?, 1, 2, 'A', 4.50, 100, 1, ?), (?, 1, 2, 'B', 4.50, 80, 1, ?), (?, 1, 2, 'C', 4.20, 120, 1, ?)", 101, now.Add(-3*time.Minute), 102, now.Add(-2*time.Minute), 103, now.Add(-1*time.Minute)).Error; err != nil {
		t.Fatalf("seed stalls failed: %v", err)
	}

	if err := db.Exec("INSERT INTO comments (stall_id, user_id, root_id, parent_id, reply_to_user_id, content, like_count, reply_count, status, created_at) VALUES (101, 1001, 0, 0, 0, 'x', 0, 0, 1, ?), (102, 1002, 0, 0, 0, 'y', 0, 0, 1, ?)", now.Add(-30*time.Second), now.Add(-20*time.Second)).Error; err != nil {
		t.Fatalf("seed comments failed: %v", err)
	}

	if err := db.Exec("INSERT INTO ratings (user_id, stall_id, score, created_at, updated_at) VALUES (1001, 101, 5, ?, ?), (1002, 102, 4, ?, ?)", now.Add(-2*time.Minute), now.Add(-2*time.Minute), now.Add(-90*time.Second), now.Add(-90*time.Second)).Error; err != nil {
		t.Fatalf("seed ratings failed: %v", err)
	}

	return repo
}

func TestMySQLRankingRepositorySameScoreCursorStability(t *testing.T) {
	repo := newRankingMySQLRepoForTest(t)
	ctx := context.Background()

	first, hasMore, err := repo.ListRankings(ctx, RankingListOptions{
		Limit:  1,
		Filter: RankingFilter{Scope: "global", Days: 30, Sort: "score_desc"},
	})
	if err != nil {
		t.Fatalf("list first ranking page failed: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("first page len = %d, want 1", len(first))
	}
	if !hasMore {
		t.Fatalf("first page should have more")
	}

	cursor := &RankingCursor{SortValue: first[0].AvgRating, LastActiveAt: first[0].LastActiveAt, StallID: first[0].StallID}
	second, _, err := repo.ListRankings(ctx, RankingListOptions{
		Limit:  2,
		Cursor: cursor,
		Filter: RankingFilter{Scope: "global", Days: 30, Sort: "score_desc"},
	})
	if err != nil {
		t.Fatalf("list second ranking page failed: %v", err)
	}
	for _, item := range second {
		if item.StallID == first[0].StallID {
			t.Fatalf("cursor pagination duplicated stall id=%d", item.StallID)
		}
	}
}
