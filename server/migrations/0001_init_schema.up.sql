CREATE TABLE IF NOT EXISTS users (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    nickname VARCHAR(64) NOT NULL,
    email VARCHAR(128) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    status TINYINT NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_users_email (email)
);

CREATE TABLE IF NOT EXISTS canteens (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(128) NOT NULL,
    campus VARCHAR(64) NOT NULL,
    status TINYINT NOT NULL DEFAULT 1,
    KEY idx_canteens_status (status)
);

CREATE TABLE IF NOT EXISTS food_types (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(64) NOT NULL
);

CREATE TABLE IF NOT EXISTS stalls (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    canteen_id BIGINT NOT NULL,
    food_type_id BIGINT NOT NULL,
    name VARCHAR(128) NOT NULL,
    avg_rating DECIMAL(4,2) NOT NULL DEFAULT 0,
    rating_count BIGINT NOT NULL DEFAULT 0,
    status TINYINT NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_stalls_canteen_id (canteen_id),
    KEY idx_stalls_food_type_id (food_type_id),
    KEY idx_stalls_status (status),
    KEY idx_stalls_avg_rating (avg_rating)
);

CREATE TABLE IF NOT EXISTS ratings (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    stall_id BIGINT NOT NULL,
    score TINYINT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_user_stall (user_id, stall_id),
    KEY idx_ratings_user_updated (user_id, updated_at, stall_id),
    KEY idx_ratings_stall_id (stall_id),
    CONSTRAINT chk_ratings_score CHECK (score BETWEEN 1 AND 5)
);

CREATE TABLE IF NOT EXISTS comments (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    stall_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    root_id BIGINT NOT NULL DEFAULT 0,
    parent_id BIGINT NOT NULL DEFAULT 0,
    reply_to_user_id BIGINT NOT NULL DEFAULT 0,
    content VARCHAR(2000) NOT NULL,
    like_count BIGINT NOT NULL DEFAULT 0,
    reply_count BIGINT NOT NULL DEFAULT 0,
    status TINYINT NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_comments_stall_created (stall_id, created_at),
    KEY idx_comments_root_created (root_id, created_at),
    KEY idx_comments_user_created (user_id, created_at),
    KEY idx_comments_status (status),
    KEY idx_comments_parent_id (parent_id),
    KEY idx_comments_reply_to_user_id (reply_to_user_id)
);

CREATE TABLE IF NOT EXISTS comment_likes (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    comment_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_comment_user (comment_id, user_id),
    KEY idx_comment_likes_comment_id (comment_id),
    KEY idx_comment_likes_user_id (user_id)
);
