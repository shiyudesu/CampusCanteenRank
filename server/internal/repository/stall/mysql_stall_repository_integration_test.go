package repository

import (
	"context"
	"os"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func openStallMySQLTestDB(t *testing.T) *gorm.DB {
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

func newStallMySQLRepoForTest(t *testing.T) *MySQLStallRepository {
	t.Helper()
	db := openStallMySQLTestDB(t)
	repo, err := NewMySQLStallRepository(db)
	if err != nil {
		t.Fatalf("new mysql stall repository failed: %v", err)
	}
	if err := db.Exec("DELETE FROM ratings").Error; err != nil {
		t.Fatalf("cleanup ratings failed: %v", err)
	}
	if err := db.Exec("DELETE FROM stalls").Error; err != nil {
		t.Fatalf("cleanup stalls failed: %v", err)
	}
	if err := db.Exec("DELETE FROM canteens").Error; err != nil {
		t.Fatalf("cleanup canteens failed: %v", err)
	}
	if err := db.Exec("INSERT INTO canteens (id, name, campus, status) VALUES (1, '一食堂', '主校区', 1)").Error; err != nil {
		t.Fatalf("seed canteen failed: %v", err)
	}
	if err := db.Exec("INSERT INTO stalls (id, canteen_id, food_type_id, name, avg_rating, rating_count, status) VALUES (101, 1, 2, '川味小炒', 0, 0, 1)").Error; err != nil {
		t.Fatalf("seed stall failed: %v", err)
	}
	return repo
}

func TestMySQLStallRepositoryUpsertUserRatingAggregatesCorrectly(t *testing.T) {
	repo := newStallMySQLRepoForTest(t)
	ctx := context.Background()

	updated, err := repo.UpsertUserRating(ctx, 1001, 101, 5)
	if err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}
	if updated.RatingCount != 1 || updated.AvgRating != 5 {
		t.Fatalf("after first upsert avg=%v count=%d, want avg=5 count=1", updated.AvgRating, updated.RatingCount)
	}

	updated, err = repo.UpsertUserRating(ctx, 1002, 101, 3)
	if err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}
	if updated.RatingCount != 2 || updated.AvgRating != 4 {
		t.Fatalf("after second upsert avg=%v count=%d, want avg=4 count=2", updated.AvgRating, updated.RatingCount)
	}

	updated, err = repo.UpsertUserRating(ctx, 1001, 101, 1)
	if err != nil {
		t.Fatalf("update existing rating failed: %v", err)
	}
	if updated.RatingCount != 2 || updated.AvgRating != 2 {
		t.Fatalf("after update avg=%v count=%d, want avg=2 count=2", updated.AvgRating, updated.RatingCount)
	}
}
