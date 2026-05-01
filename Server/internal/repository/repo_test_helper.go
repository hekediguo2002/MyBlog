//go:build test || !ignore
// +build test !ignore

package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wjr/blog/server/internal/model"
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
