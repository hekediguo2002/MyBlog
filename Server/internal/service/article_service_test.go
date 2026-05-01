package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/model"
	"github.com/wjr/blog/server/internal/repository"
)

type memCounter struct{ inc map[uint64]int64 }

func (m *memCounter) Inc(ctx context.Context, id uint64) error {
	m.inc[id]++
	return nil
}
func (m *memCounter) GetIncrement(ctx context.Context, id uint64) (int64, error) {
	return m.inc[id], nil
}
func (m *memCounter) DirtyMembers(ctx context.Context) ([]string, error) { return nil, nil }
func (m *memCounter) DrainIncrement(ctx context.Context, id uint64) (int64, error) {
	v := m.inc[id]
	m.inc[id] = 0
	return v, nil
}
func (m *memCounter) Ack(ctx context.Context, ids []uint64) error { return nil }
func (m *memCounter) Restore(ctx context.Context, id uint64, n int64) error {
	m.inc[id] += n
	return nil
}

func newArticleSvc(t *testing.T) (*ArticleService, *memCounter) {
	t.Helper()
	db := newTestDB(t)
	require.NoError(t, db.Create(&model.User{ID: 1, Username: "alice", Name: "Alice", PasswordHash: "x"}).Error)
	require.NoError(t, db.Create(&model.User{ID: 2, Username: "bob", Name: "Bob", PasswordHash: "x"}).Error)
	cnt := &memCounter{inc: map[uint64]int64{}}
	svc := NewArticleService(
		repository.NewArticleRepo(db),
		repository.NewTagRepo(db),
		repository.NewUserRepo(db),
		cnt,
	)
	return svc, cnt
}

func TestArticleService_Create_AssignsTagsAndSummary(t *testing.T) {
	svc, _ := newArticleSvc(t)
	ctx := context.Background()
	a, err := svc.Create(ctx, 1, CreateArticleInput{
		Title:   "首篇",
		Content: "# 标题\n\n这是 **正文** 内容,放一些字。",
		Tags:    []string{"go", "go", "  rust  "},
	})
	require.NoError(t, err)
	require.NotZero(t, a.ID)
	require.Equal(t, "首篇", a.Title)
	require.NotEmpty(t, a.Summary)
	require.NotContains(t, a.Summary, "#")
	require.Len(t, a.Tags, 2)
}

func TestArticleService_Update_ForbidsOtherUser(t *testing.T) {
	svc, _ := newArticleSvc(t)
	ctx := context.Background()
	a, err := svc.Create(ctx, 1, CreateArticleInput{
		Title: "alice 的文章", Content: "正文正文正文正文正文正文正文正文正文正文",
	})
	require.NoError(t, err)

	_, err = svc.Update(ctx, 2, a.ID, UpdateArticleInput{Title: "改", Content: "正文正文正文正文正文正文正文正文正文正文"})
	require.Error(t, err)
	var ae *apperr.AppErr
	require.True(t, errors.As(err, &ae))
	require.Equal(t, apperr.CodeForbidden, ae.Code)
}

func TestArticleService_GetByID_MergesRedisIncrement(t *testing.T) {
	svc, cnt := newArticleSvc(t)
	ctx := context.Background()
	a, err := svc.Create(ctx, 1, CreateArticleInput{Title: "T", Content: "正文 contents 12345"})
	require.NoError(t, err)

	require.NoError(t, cnt.Inc(ctx, a.ID))
	require.NoError(t, cnt.Inc(ctx, a.ID))
	require.NoError(t, cnt.Inc(ctx, a.ID))

	got, err := svc.GetByID(ctx, a.ID, true)
	require.NoError(t, err)
	require.Equal(t, int64(4), got.ViewCount)
}

func TestArticleService_List_Pagination(t *testing.T) {
	svc, _ := newArticleSvc(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, err := svc.Create(ctx, 1, CreateArticleInput{Title: "文章", Content: "正文正文正文正文正文正文正文正文正文正文"})
		require.NoError(t, err)
	}

	items, total, err := svc.List(ctx, ListArticlesInput{Page: 1, Size: 2})
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Len(t, items, 2)
	require.Empty(t, items[0].Content)
}

func TestArticleService_Delete_OwnerOnly(t *testing.T) {
	svc, _ := newArticleSvc(t)
	ctx := context.Background()
	a, err := svc.Create(ctx, 1, CreateArticleInput{Title: "T", Content: "正文正文正文正文正文正文正文正文正文正文"})
	require.NoError(t, err)

	err = svc.Delete(ctx, 2, a.ID)
	var ae *apperr.AppErr
	require.True(t, errors.As(err, &ae))
	require.Equal(t, apperr.CodeForbidden, ae.Code)

	err = svc.Delete(ctx, 1, a.ID)
	require.NoError(t, err)

	_, err = svc.GetByID(ctx, a.ID, false)
	require.True(t, errors.As(err, &ae))
	require.Equal(t, apperr.CodeNotFound, ae.Code)
}
