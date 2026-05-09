CREATE TABLE IF NOT EXISTS users (
  id            BIGSERIAL PRIMARY KEY,
  username      VARCHAR(32)     NOT NULL,
  password_hash CHAR(60)        NOT NULL,
  name          VARCHAR(64)     NOT NULL,
  created_at    TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX uk_username ON users(username);

CREATE TABLE IF NOT EXISTS articles (
  id          BIGSERIAL PRIMARY KEY,
  user_id     BIGINT           NOT NULL,
  title       VARCHAR(200)     NOT NULL,
  content     TEXT             NOT NULL,
  summary     VARCHAR(500)     NOT NULL DEFAULT '',
  view_count  BIGINT           NOT NULL DEFAULT 0,
  status      SMALLINT         NOT NULL DEFAULT 1,
  created_at  TIMESTAMP        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  TIMESTAMP        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at  TIMESTAMP        NULL
);

CREATE INDEX idx_user_id ON articles(user_id);
CREATE INDEX idx_status_created ON articles(status, created_at);
CREATE INDEX idx_deleted_at ON articles(deleted_at);

CREATE TABLE IF NOT EXISTS tags (
  id   BIGSERIAL PRIMARY KEY,
  name VARCHAR(32) NOT NULL
);

CREATE UNIQUE INDEX uk_name ON tags(name);

CREATE TABLE IF NOT EXISTS article_tags (
  article_id BIGINT NOT NULL,
  tag_id     BIGINT NOT NULL,
  PRIMARY KEY (article_id, tag_id)
);

CREATE INDEX idx_tag_article ON article_tags(tag_id, article_id);
