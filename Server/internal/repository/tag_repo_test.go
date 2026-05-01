package repository

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wjr/blog/server/internal/model"
)

func TestTagRepo_EnsureMany_DedupAndUpsert(t *testing.T) {
	db := newTestDB(t)
	repo := NewTagRepo(db)
	ctx := context.Background()

	tags, err := repo.EnsureMany(ctx, []string{"go", "blog", "go"})
	require.NoError(t, err)
	require.Len(t, tags, 2)
	names := []string{tags[0].Name, tags[1].Name}
	sort.Strings(names)
	require.Equal(t, []string{"blog", "go"}, names)

	again, err := repo.EnsureMany(ctx, []string{"go"})
	require.NoError(t, err)
	require.Len(t, again, 1)
	require.Equal(t, findTagByName(tags, "go").ID, again[0].ID)
	_ = again
}

func TestTagRepo_ListWithCount(t *testing.T) {
	db := newTestDB(t)
	repo := NewTagRepo(db)
	ctx := context.Background()

	tags, err := repo.EnsureMany(ctx, []string{"go", "blog"})
	require.NoError(t, err)
	goTag := *findTagByName(tags, "go")
	blogTag := *findTagByName(tags, "blog")
	require.NoError(t, db.Create(&model.Article{UserID: 1, Title: "a", Content: "c", Status: 1, Tags: []model.Tag{goTag}}).Error)
	require.NoError(t, db.Create(&model.Article{UserID: 1, Title: "b", Content: "c", Status: 1, Tags: []model.Tag{goTag, blogTag}}).Error)

	rows, err := repo.ListWithCount(ctx)
	require.NoError(t, err)
	got := map[string]int{}
	for _, r := range rows {
		got[r.Name] = r.ArticleCount
	}
	require.Equal(t, 2, got["go"])
	require.Equal(t, 1, got["blog"])
}

func findTagByName(ts []model.Tag, name string) *model.Tag {
	for i := range ts {
		if ts[i].Name == name {
			return &ts[i]
		}
	}
	return nil
}
