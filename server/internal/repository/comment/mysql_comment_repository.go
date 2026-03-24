package repository

import (
	"context"
	"errors"
	"time"

	model "CampusCanteenRank/server/internal/model/comment"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type mysqlCommentRecord struct {
	ID            int64     `gorm:"column:id;primaryKey;autoIncrement"`
	StallID       int64     `gorm:"column:stall_id;not null;index:idx_stall_created,priority:1;index"`
	UserID        int64     `gorm:"column:user_id;not null;index:idx_user_created,priority:1;index"`
	RootID        int64     `gorm:"column:root_id;not null;default:0;index:idx_root_created,priority:1;index"`
	ParentID      int64     `gorm:"column:parent_id;not null;default:0;index"`
	ReplyToUserID int64     `gorm:"column:reply_to_user_id;not null;default:0;index"`
	Content       string    `gorm:"column:content;type:varchar(2000);not null"`
	LikeCount     int64     `gorm:"column:like_count;not null;default:0"`
	ReplyCount    int64     `gorm:"column:reply_count;not null;default:0"`
	Status        int8      `gorm:"column:status;type:tinyint;not null;default:1;index"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime;index:idx_stall_created,priority:2;index:idx_root_created,priority:2;index:idx_user_created,priority:2"`
}

func (mysqlCommentRecord) TableName() string {
	return "comments"
}

type mysqlCommentLikeRecord struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	CommentID int64     `gorm:"column:comment_id;not null;index:uk_comment_user,unique,priority:1;index"`
	UserID    int64     `gorm:"column:user_id;not null;index:uk_comment_user,unique,priority:2;index"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (mysqlCommentLikeRecord) TableName() string {
	return "comment_likes"
}

type MySQLCommentRepository struct {
	db *gorm.DB
}

func NewMySQLCommentRepository(db *gorm.DB) (*MySQLCommentRepository, error) {
	if db == nil {
		return nil, errors.New("nil mysql db")
	}
	if err := db.AutoMigrate(&mysqlCommentRecord{}, &mysqlCommentLikeRecord{}); err != nil {
		return nil, err
	}
	return &MySQLCommentRepository{db: db}, nil
}

func (r *MySQLCommentRepository) Create(ctx context.Context, comment *model.Comment) error {
	rec := mysqlCommentRecord{
		StallID:       comment.StallID,
		UserID:        comment.UserID,
		RootID:        comment.RootID,
		ParentID:      comment.ParentID,
		ReplyToUserID: comment.ReplyToUserID,
		Content:       comment.Content,
		LikeCount:     comment.LikeCount,
		ReplyCount:    comment.ReplyCount,
		Status:        comment.Status,
	}
	if err := r.db.WithContext(ctx).Create(&rec).Error; err != nil {
		return err
	}
	comment.ID = rec.ID
	comment.CreatedAt = rec.CreatedAt
	comment.Status = rec.Status
	return nil
}

func (r *MySQLCommentRepository) CreateReplyAndIncrementRoot(ctx context.Context, reply *model.Comment, rootID int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rootRec mysqlCommentRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND status = ? AND root_id = 0 AND parent_id = 0", rootID, 1).
			First(&rootRec).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}

		rec := mysqlCommentRecord{
			StallID:       reply.StallID,
			UserID:        reply.UserID,
			RootID:        reply.RootID,
			ParentID:      reply.ParentID,
			ReplyToUserID: reply.ReplyToUserID,
			Content:       reply.Content,
			LikeCount:     reply.LikeCount,
			ReplyCount:    reply.ReplyCount,
			Status:        reply.Status,
		}
		if err := tx.Create(&rec).Error; err != nil {
			return err
		}

		if err := tx.Model(&mysqlCommentRecord{}).
			Where("id = ?", rootID).
			Update("reply_count", gorm.Expr("reply_count + 1")).Error; err != nil {
			return err
		}

		reply.ID = rec.ID
		reply.CreatedAt = rec.CreatedAt
		reply.Status = rec.Status
		return nil
	})
}

func (r *MySQLCommentRepository) GetByID(ctx context.Context, commentID int64) (*model.Comment, error) {
	var rec mysqlCommentRecord
	err := r.db.WithContext(ctx).Where("id = ? AND status = ?", commentID, 1).First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return toCommentModel(rec), nil
}

func (r *MySQLCommentRepository) IncrementRootReplyCount(ctx context.Context, rootID int64) error {
	result := r.db.WithContext(ctx).
		Model(&mysqlCommentRecord{}).
		Where("id = ? AND status = ? AND root_id = 0 AND parent_id = 0", rootID, 1).
		Update("reply_count", gorm.Expr("reply_count + 1"))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MySQLCommentRepository) Like(ctx context.Context, userID int64, commentID int64) (int64, error) {
	var likeCount int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var commentRec mysqlCommentRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND status = ?", commentID, 1).
			First(&commentRec).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}

		likeRec := mysqlCommentLikeRecord{CommentID: commentID, UserID: userID}
		err := tx.Create(&likeRec).Error
		if err != nil {
			if isMySQLDuplicate(err) {
				likeCount = commentRec.LikeCount
				return nil
			}
			return err
		}

		commentRec.LikeCount++
		if err := tx.Model(&mysqlCommentRecord{}).
			Where("id = ?", commentID).
			Update("like_count", commentRec.LikeCount).Error; err != nil {
			return err
		}
		likeCount = commentRec.LikeCount
		return nil
	})
	if err != nil {
		return 0, err
	}
	return likeCount, nil
}

func (r *MySQLCommentRepository) Unlike(ctx context.Context, userID int64, commentID int64) (int64, error) {
	var likeCount int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var commentRec mysqlCommentRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND status = ?", commentID, 1).
			First(&commentRec).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}

		result := tx.Where("comment_id = ? AND user_id = ?", commentID, userID).Delete(&mysqlCommentLikeRecord{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 && commentRec.LikeCount > 0 {
			commentRec.LikeCount--
			if err := tx.Model(&mysqlCommentRecord{}).
				Where("id = ?", commentID).
				Update("like_count", commentRec.LikeCount).Error; err != nil {
				return err
			}
		}
		likeCount = commentRec.LikeCount
		return nil
	})
	if err != nil {
		return 0, err
	}
	return likeCount, nil
}

func (r *MySQLCommentRepository) HasLiked(ctx context.Context, userID int64, commentID int64) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&mysqlCommentRecord{}).
		Where("id = ? AND status = ?", commentID, 1).
		Count(&count).Error; err != nil {
		return false, err
	}
	if count == 0 {
		return false, ErrNotFound
	}

	var likeCount int64
	if err := r.db.WithContext(ctx).
		Model(&mysqlCommentLikeRecord{}).
		Where("comment_id = ? AND user_id = ?", commentID, userID).
		Count(&likeCount).Error; err != nil {
		return false, err
	}
	return likeCount > 0, nil
}

func (r *MySQLCommentRepository) HasLikedBatch(ctx context.Context, userID int64, commentIDs []int64) (map[int64]bool, error) {
	result := make(map[int64]bool, len(commentIDs))
	if len(commentIDs) == 0 {
		return result, nil
	}

	for _, commentID := range commentIDs {
		if commentID <= 0 {
			continue
		}
		result[commentID] = false
	}

	var rows []mysqlCommentLikeRecord
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND comment_id IN ?", userID, commentIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.CommentID] = true
	}
	return result, nil
}

func (r *MySQLCommentRepository) ListTopLevelByStall(ctx context.Context, options CommentListOptions) ([]model.Comment, bool, error) {
	limit := options.Limit
	if limit <= 0 {
		limit = 20
	}

	query := r.db.WithContext(ctx).
		Model(&mysqlCommentRecord{}).
		Where("status = ? AND stall_id = ? AND root_id = 0 AND parent_id = 0", 1, options.StallID)
	if options.Cursor != nil {
		query = query.Where("(created_at < ?) OR (created_at = ? AND id < ?)", options.Cursor.CreatedAt, options.Cursor.CreatedAt, options.Cursor.ID)
	}

	var records []mysqlCommentRecord
	if err := query.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&records).Error; err != nil {
		return nil, false, err
	}
	return trimAndMapComments(records, limit), len(records) > limit, nil
}

func (r *MySQLCommentRepository) ListRepliesByRoot(
	ctx context.Context,
	rootCommentID int64,
	limit int,
	cursor *CommentCursor,
) ([]model.Comment, bool, error) {
	if limit <= 0 {
		limit = 20
	}

	query := r.db.WithContext(ctx).
		Model(&mysqlCommentRecord{}).
		Where("status = ? AND root_id = ? AND parent_id <> 0", 1, rootCommentID)
	if cursor != nil {
		query = query.Where("(created_at < ?) OR (created_at = ? AND id < ?)", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}

	var records []mysqlCommentRecord
	if err := query.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&records).Error; err != nil {
		return nil, false, err
	}
	return trimAndMapComments(records, limit), len(records) > limit, nil
}

func (r *MySQLCommentRepository) ListByUser(ctx context.Context, userID int64, limit int, cursor *CommentCursor) ([]model.Comment, bool, error) {
	if limit <= 0 {
		limit = 20
	}

	query := r.db.WithContext(ctx).
		Model(&mysqlCommentRecord{}).
		Where("status = ? AND user_id = ?", 1, userID)
	if cursor != nil {
		query = query.Where("(created_at < ?) OR (created_at = ? AND id < ?)", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}

	var records []mysqlCommentRecord
	if err := query.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&records).Error; err != nil {
		return nil, false, err
	}
	return trimAndMapComments(records, limit), len(records) > limit, nil
}

func trimAndMapComments(records []mysqlCommentRecord, limit int) []model.Comment {
	if len(records) > limit {
		records = records[:limit]
	}
	out := make([]model.Comment, 0, len(records))
	for _, rec := range records {
		out = append(out, *toCommentModel(rec))
	}
	return out
}

func toCommentModel(rec mysqlCommentRecord) *model.Comment {
	return &model.Comment{
		ID:            rec.ID,
		StallID:       rec.StallID,
		UserID:        rec.UserID,
		RootID:        rec.RootID,
		ParentID:      rec.ParentID,
		ReplyToUserID: rec.ReplyToUserID,
		Content:       rec.Content,
		LikeCount:     rec.LikeCount,
		ReplyCount:    rec.ReplyCount,
		Status:        rec.Status,
		CreatedAt:     rec.CreatedAt,
	}
}

func isMySQLDuplicate(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062
	}
	return false
}
