package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/model"
)

func seedUser(t *testing.T, repo UserRepo) *model.User {
	t.Helper()
	u := &model.User{Username: "alice", PasswordHash: "h", Name: "A"}
	require.NoError(t, repo.Create(context.Background(), u))
	return u
}

func TestArticleRepo_CreateWithTags_AndFind(t *testing.T) {
	db := newTestDB(t)
	uRepo := NewUserRepo(db)
	tRepo := NewTagRepo(db)
	aRepo := NewArticleRepo(db)
	ctx := context.Background()

	u := seedUser(t, uRepo)
	tags, err := tRepo.EnsureMany(ctx, []string{"go", "blog"})
	require.NoError(t, err)

	art := &model.Article{UserID: u.ID, Title: "Hello", Content: "# md", Status: 1}
	require.NoError(t, aRepo.Create(ctx, art, tags))
	require.Greater(t, art.ID, uint64(0))

	got, err := aRepo.FindByID(ctx, art.ID)
	require.NoError(t, err)
	require.Equal(t, "Hello", got.Title)
	require.Len(t, got.Tags, 2)
	require.Equal(t, u.ID, got.Author.ID)
}

func TestArticleRepo_Update_ReplacesTags(t *testing.T) {
	db := newTestDB(t)
	uRepo := NewUserRepo(db)
	tRepo := NewTagRepo(db)
	aRepo := NewArticleRepo(db)
	ctx := context.Background()

	u := seedUser(t, uRepo)
	t1, _ := tRepo.EnsureMany(ctx, []string{"go", "blog"})
	art := &model.Article{UserID: u.ID, Title: "T", Content: "c", Status: 1}
	require.NoError(t, aRepo.Create(ctx, art, t1))

	t2, _ := tRepo.EnsureMany(ctx, []string{"redis"})
	art.Title = "T2"
	art.Content = "c2"
	require.NoError(t, aRepo.Update(ctx, art, t2))

	got, _ := aRepo.FindByID(ctx, art.ID)
	require.Equal(t, "T2", got.Title)
	require.Len(t, got.Tags, 1)
	require.Equal(t, "redis", got.Tags[0].Name)
}

func TestArticleRepo_SoftDelete(t *testing.T) {
	db := newTestDB(t)
	u := seedUser(t, NewUserRepo(db))
	aRepo := NewArticleRepo(db)
	ctx := context.Background()

	art := &model.Article{UserID: u.ID, Title: "T", Content: "c", Status: 1}
	require.NoError(t, aRepo.Create(ctx, art, nil))
	require.NoError(t, aRepo.SoftDelete(ctx, art.ID))

	_, err := aRepo.FindByID(ctx, art.ID)
	var ae *apperr.AppErr
	require.True(t, errors.As(err, &ae))
	require.Equal(t, apperr.CodeNotFound, ae.Code)
}

func TestArticleRepo_List_FilterByUser(t *testing.T) {
	db := newTestDB(t)
	uRepo := NewUserRepo(db)
	aRepo := NewArticleRepo(db)
	ctx := context.Background()

	u1 := &model.User{Username: "u1", PasswordHash: "h", Name: "U1"}
	u2 := &model.User{Username: "u2", PasswordHash: "h", Name: "U2"}
	require.NoError(t, uRepo.Create(ctx, u1))
	require.NoError(t, uRepo.Create(ctx, u2))

	for i := 0; i < 3; i++ {
		require.NoError(t, aRepo.Create(ctx, &model.Article{UserID: u1.ID, Title: "a", Content: "c", Status: 1}, nil))
	}
	require.NoError(t, aRepo.Create(ctx, &model.Article{UserID: u2.ID, Title: "b", Content: "c", Status: 1}, nil))

	rows, total, err := aRepo.List(ctx, ListQuery{Page: 1, Size: 10, UserID: u1.ID})
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, rows, 3)
}

func TestArticleRepo_List_FilterByTag(t *testing.T) {
	db := newTestDB(t)
	uRepo := NewUserRepo(db)
	tRepo := NewTagRepo(db)
	aRepo := NewArticleRepo(db)
	ctx := context.Background()

	u := seedUser(t, uRepo)
	tags, _ := tRepo.EnsureMany(ctx, []string{"go", "blog"})
	goTag := findTagByName(tags, "go")
	require.NotNil(t, goTag)
	require.NoError(t, aRepo.Create(ctx, &model.Article{UserID: u.ID, Title: "with-go", Content: "c", Status: 1}, []model.Tag{*goTag}))
	require.NoError(t, aRepo.Create(ctx, &model.Article{UserID: u.ID, Title: "no-go",   Content: "c", Status: 1}, nil))

	rows, total, err := aRepo.List(ctx, ListQuery{Page: 1, Size: 10, Tag: "go"})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, "with-go", rows[0].Title)
}
