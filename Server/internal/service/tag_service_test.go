package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wjr/blog/server/internal/model"
	"github.com/wjr/blog/server/internal/repository"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{NowFunc: func() time.Time { return time.Now().UTC() }})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Article{}, &model.Tag{}))
	t.Cleanup(func() {
		_ = db.Migrator().DropTable(&model.User{}, &model.Article{}, &model.Tag{}, "article_tags")
	})
	return db
}

func TestTagService_List_OrdersByCountDesc(t *testing.T) {
	db := newTestDB(t)
	svc := NewTagService(repository.NewTagRepo(db))
	ctx := context.Background()

	require.NoError(t, db.Create(&model.Tag{Name: "go"}).Error)
	require.NoError(t, db.Create(&model.Tag{Name: "rust"}).Error)
	require.NoError(t, db.Create(&model.Tag{Name: "ai"}).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO article_tags(article_id, tag_id) VALUES (1,1),(2,1),(3,2)",
	).Error)

	out, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, out, 3)
	require.Equal(t, "go", out[0].Name)
	require.Equal(t, int64(2), out[0].ArticleCount)
	require.Equal(t, "rust", out[1].Name)
	require.Equal(t, int64(1), out[1].ArticleCount)
	require.Equal(t, "ai", out[2].Name)
	require.Equal(t, int64(0), out[2].ArticleCount)
}
