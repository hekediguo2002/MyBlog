SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS users (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  username      VARCHAR(32)     NOT NULL,
  password_hash CHAR(60)        NOT NULL,
  name          VARCHAR(64)     NOT NULL,
  created_at    DATETIME        NOT NULL,
  updated_at    DATETIME        NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS articles (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id     BIGINT UNSIGNED NOT NULL,
  title       VARCHAR(200)    NOT NULL,
  content     MEDIUMTEXT      NOT NULL,
  summary     VARCHAR(500)    NOT NULL DEFAULT '',
  view_count  BIGINT UNSIGNED NOT NULL DEFAULT 0,
  status      TINYINT         NOT NULL DEFAULT 1,
  created_at  DATETIME        NOT NULL,
  updated_at  DATETIME        NOT NULL,
  deleted_at  DATETIME        NULL,
  PRIMARY KEY (id),
  KEY idx_user_id (user_id),
  KEY idx_status_created (status, created_at),
  KEY idx_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS tags (
  id   BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name VARCHAR(32)     NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS article_tags (
  article_id BIGINT UNSIGNED NOT NULL,
  tag_id     BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (article_id, tag_id),
  KEY idx_tag_article (tag_id, article_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
