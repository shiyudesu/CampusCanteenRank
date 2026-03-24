package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"CampusCanteenRank/server/internal/migration"
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
	if err := migration.ApplySQLMigrations(db); err != nil {
		t.Skipf("apply sql migrations failed for integration test: %v", err)
	}
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

	if err := db.Exec("INSERT INTO canteens (id, name, campus, status) VALUES (1, '一食堂', '主校区', 1), (2, '二食堂', '南校区', 1)").Error; err != nil {
		t.Fatalf("seed canteen failed: %v", err)
	}
	if err := db.Exec("INSERT INTO food_types (id, name) VALUES (2, '川菜'), (3, '面食')").Error; err != nil {
		t.Fatalf("seed food type failed: %v", err)
	}

	now := time.Now().UTC()
	if err := db.Exec(
		"INSERT INTO stalls (id, canteen_id, food_type_id, name, avg_rating, rating_count, status, created_at) VALUES (?, 1, 2, 'A', 4.50, 100, 1, ?), (?, 1, 3, 'B', 4.50, 80, 1, ?), (?, 2, 2, 'C', 4.20, 120, 1, ?)",
		101,
		now.Add(-3*time.Minute),
		102,
		now.Add(-2*time.Minute),
		103,
		now.Add(-1*time.Minute),
	).Error; err != nil {
		t.Fatalf("seed stalls failed: %v", err)
	}

	if err := db.Exec(
		"INSERT INTO comments (stall_id, user_id, root_id, parent_id, reply_to_user_id, content, like_count, reply_count, status, created_at) VALUES (101, 1001, 0, 0, 0, 'x1', 0, 0, 1, ?), (101, 1002, 0, 0, 0, 'x2', 0, 0, 1, ?), (102, 1003, 0, 0, 0, 'old', 0, 0, 1, ?), (103, 1004, 0, 0, 0, 'z', 0, 0, 1, ?)",
		now.Add(-30*time.Second),
		now.Add(-20*time.Second),
		now.Add(-40*24*time.Hour),
		now.Add(-10*time.Second),
	).Error; err != nil {
		t.Fatalf("seed comments failed: %v", err)
	}

	if err := db.Exec(
		"INSERT INTO ratings (user_id, stall_id, score, created_at, updated_at) VALUES (1001, 101, 5, ?, ?), (1002, 102, 4, ?, ?), (1003, 103, 5, ?, ?)",
		now.Add(-2*time.Minute),
		now.Add(-2*time.Minute),
		now.Add(-45*24*time.Hour),
		now.Add(-45*24*time.Hour),
		now.Add(-90*time.Second),
		now.Add(-90*time.Second),
	).Error; err != nil {
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
	second, secondHasMore, err := repo.ListRankings(ctx, RankingListOptions{
		Limit:  2,
		Cursor: cursor,
		Filter: RankingFilter{Scope: "global", Days: 30, Sort: "score_desc"},
	})
	if err != nil {
		t.Fatalf("list second ranking page failed: %v", err)
	}
	if len(second) == 0 {
		t.Fatalf("second page should not be empty")
	}
	if secondHasMore {
		t.Fatalf("second page should be terminal with current seeds")
	}
	if second[0].StallID != 102 {
		t.Fatalf("second page first stall id = %d, want 102", second[0].StallID)
	}
	for _, item := range second {
		if item.StallID == first[0].StallID {
			t.Fatalf("cursor pagination duplicated stall id=%d", item.StallID)
		}
	}
}

func TestMySQLRankingRepositoryHotCursorPagination(t *testing.T) {
	repo := newRankingMySQLRepoForTest(t)
	ctx := context.Background()

	first, hasMore, err := repo.ListRankings(ctx, RankingListOptions{
		Limit:  1,
		Filter: RankingFilter{Scope: "global", Days: 30, Sort: "hot_desc"},
	})
	if err != nil {
		t.Fatalf("list first hot ranking page failed: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("first hot page len = %d, want 1", len(first))
	}
	if !hasMore {
		t.Fatalf("first hot page should have more")
	}

	cursor := &RankingCursor{SortValue: first[0].HotScore, LastActiveAt: first[0].LastActiveAt, StallID: first[0].StallID}
	second, _, err := repo.ListRankings(ctx, RankingListOptions{
		Limit:  2,
		Cursor: cursor,
		Filter: RankingFilter{Scope: "global", Days: 30, Sort: "hot_desc"},
	})
	if err != nil {
		t.Fatalf("list second hot ranking page failed: %v", err)
	}
	if len(second) == 0 {
		t.Fatalf("second hot page should not be empty")
	}
	for _, item := range second {
		if item.StallID == first[0].StallID {
			t.Fatalf("hot cursor pagination duplicated stall id=%d", item.StallID)
		}
		if item.HotScore > first[0].HotScore {
			t.Fatalf("hot cursor page contains higher hotScore item: got %f > %f", item.HotScore, first[0].HotScore)
		}
	}
}

func TestMySQLRankingRepositoryScopeAndFoodTypeFilters(t *testing.T) {
	repo := newRankingMySQLRepoForTest(t)
	ctx := context.Background()

	byCanteen, _, err := repo.ListRankings(ctx, RankingListOptions{
		Limit: 20,
		Filter: RankingFilter{
			Scope:   "canteen",
			ScopeID: 1,
			Days:    30,
			Sort:    "score_desc",
		},
	})
	if err != nil {
		t.Fatalf("list rankings by canteen failed: %v", err)
	}
	if len(byCanteen) == 0 {
		t.Fatalf("canteen filter should not be empty")
	}
	for _, item := range byCanteen {
		if item.CanteenID != 1 {
			t.Fatalf("unexpected canteen id=%d in canteen scope result", item.CanteenID)
		}
	}

	byScopeFoodType, _, err := repo.ListRankings(ctx, RankingListOptions{
		Limit: 20,
		Filter: RankingFilter{
			Scope:   "foodType",
			ScopeID: 2,
			Days:    30,
			Sort:    "score_desc",
		},
	})
	if err != nil {
		t.Fatalf("list rankings by scope food type failed: %v", err)
	}
	if len(byScopeFoodType) == 0 {
		t.Fatalf("food type scope filter should not be empty")
	}
	for _, item := range byScopeFoodType {
		if item.FoodTypeID != 2 {
			t.Fatalf("unexpected food type id=%d in foodType scope result", item.FoodTypeID)
		}
	}

	byExtraFoodType, _, err := repo.ListRankings(ctx, RankingListOptions{
		Limit: 20,
		Filter: RankingFilter{
			Scope:      "global",
			FoodTypeID: 3,
			Days:       30,
			Sort:       "score_desc",
		},
	})
	if err != nil {
		t.Fatalf("list rankings by extra food type filter failed: %v", err)
	}
	if len(byExtraFoodType) != 1 {
		t.Fatalf("extra food type filter len = %d, want 1", len(byExtraFoodType))
	}
	if byExtraFoodType[0].StallID != 102 {
		t.Fatalf("extra food type filter stall id = %d, want 102", byExtraFoodType[0].StallID)
	}
}

func TestMySQLRankingRepositoryHotSortAndHotScoreField(t *testing.T) {
	repo := newRankingMySQLRepoForTest(t)
	ctx := context.Background()

	items, _, err := repo.ListRankings(ctx, RankingListOptions{
		Limit: 20,
		Filter: RankingFilter{
			Scope: "global",
			Days:  30,
			Sort:  "hot_desc",
		},
	})
	if err != nil {
		t.Fatalf("list rankings by hot desc failed: %v", err)
	}
	if len(items) < 2 {
		t.Fatalf("hot sort result should contain at least 2 items")
	}
	if items[0].StallID != 101 {
		t.Fatalf("hot desc first stall id = %d, want 101", items[0].StallID)
	}

	scoreItems, _, err := repo.ListRankings(ctx, RankingListOptions{
		Limit: 20,
		Filter: RankingFilter{
			Scope: "global",
			Days:  30,
			Sort:  "score_desc",
		},
	})
	if err != nil {
		t.Fatalf("list rankings by score desc failed: %v", err)
	}

	var hot101, hot102 float64
	for _, item := range scoreItems {
		switch item.StallID {
		case 101:
			hot101 = item.HotScore
		case 102:
			hot102 = item.HotScore
		}
	}
	if hot101 <= hot102 {
		t.Fatalf("hot score should reflect recent review activity, hot101=%f hot102=%f", hot101, hot102)
	}
}

func TestMySQLRankingRepositoryDaysWindowAffectsReviewCount(t *testing.T) {
	repo := newRankingMySQLRepoForTest(t)
	ctx := context.Background()

	items30, _, err := repo.ListRankings(ctx, RankingListOptions{
		Limit: 20,
		Filter: RankingFilter{
			Scope: "global",
			Days:  30,
			Sort:  "hot_desc",
		},
	})
	if err != nil {
		t.Fatalf("list rankings with 30-day window failed: %v", err)
	}

	items90, _, err := repo.ListRankings(ctx, RankingListOptions{
		Limit: 20,
		Filter: RankingFilter{
			Scope: "global",
			Days:  90,
			Sort:  "hot_desc",
		},
	})
	if err != nil {
		t.Fatalf("list rankings with 90-day window failed: %v", err)
	}

	review30 := int64(-1)
	review90 := int64(-1)
	for _, item := range items30 {
		if item.StallID == 102 {
			review30 = item.ReviewCount
			break
		}
	}
	for _, item := range items90 {
		if item.StallID == 102 {
			review90 = item.ReviewCount
			break
		}
	}
	if review30 != 0 {
		t.Fatalf("30-day review count for stall 102 = %d, want 0", review30)
	}
	if review90 != 1 {
		t.Fatalf("90-day review count for stall 102 = %d, want 1", review90)
	}
}
