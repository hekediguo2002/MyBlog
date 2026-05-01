package service

import (
	"context"
	"strings"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/model"
	"github.com/wjr/blog/server/internal/pkg/markdownx"
	"github.com/wjr/blog/server/internal/repository"
)

type CreateArticleInput struct {
	Title   string
	Content string
	Tags    []string
}

type UpdateArticleInput struct {
	Title   string
	Content string
	Tags    []string
}

type ArticleView struct {
	ID         uint64   `json:"id"`
	Title      string   `json:"title"`
	Content    string   `json:"content,omitempty"`
	Summary    string   `json:"summary,omitempty"`
	Tags       []string `json:"tags"`
	UserID     uint64   `json:"user_id"`
	AuthorName string   `json:"author_name,omitempty"`
	ViewCount  int64    `json:"view_count"`
	CreatedAt  int64    `json:"created_at"`
	UpdatedAt  int64    `json:"updated_at"`
}

type ListArticlesInput struct {
	Page   int
	Size   int
	Tag    string
	UserID uint64
}

type ArticleService struct {
	articles repository.ArticleRepo
	tags     repository.TagRepo
	users    repository.UserRepo
	counter  repository.CounterRepo
}

func NewArticleService(
	articles repository.ArticleRepo,
	tags repository.TagRepo,
	users repository.UserRepo,
	counter repository.CounterRepo,
) *ArticleService {
	return &ArticleService{articles: articles, tags: tags, users: users, counter: counter}
}

const (
	titleMin   = 1
	titleMax   = 200
	contentMin = 10
	contentMax = 100000
	tagsMax    = 5
	tagMaxLen  = 32
)

func validateArticleInput(title, content string, tags []string) error {
	t := strings.TrimSpace(title)
	if len([]rune(t)) < titleMin || len([]rune(t)) > titleMax {
		return apperr.New(apperr.CodeInvalidParam, "标题长度需 1-200")
	}
	if len([]rune(content)) < contentMin || len([]rune(content)) > contentMax {
		return apperr.New(apperr.CodeInvalidParam, "正文长度需 10-100000")
	}
	if len(tags) > tagsMax {
		return apperr.New(apperr.CodeInvalidParam, "标签最多 5 个")
	}
	for _, tag := range tags {
		if len([]rune(tag)) > tagMaxLen {
			return apperr.New(apperr.CodeInvalidParam, "标签过长")
		}
	}
	return nil
}

func normalizeTags(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, t := range in {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

func (s *ArticleService) Create(ctx context.Context, userID uint64, in CreateArticleInput) (*ArticleView, error) {
	tags := normalizeTags(in.Tags)
	if err := validateArticleInput(in.Title, in.Content, tags); err != nil {
		return nil, err
	}
	tagModels, err := s.tags.EnsureMany(ctx, tags)
	if err != nil {
		return nil, err
	}
	a := &model.Article{
		UserID:  userID,
		Title:   strings.TrimSpace(in.Title),
		Content: in.Content,
		Summary: markdownx.Summary(in.Content, 200),
	}
	if err := s.articles.Create(ctx, a, tagModels); err != nil {
		return nil, err
	}
	return s.viewOf(ctx, a, false)
}

func (s *ArticleService) Update(ctx context.Context, userID uint64, id uint64, in UpdateArticleInput) (*ArticleView, error) {
	a, err := s.articles.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if a.UserID != userID {
		return nil, apperr.New(apperr.CodeForbidden, "无权操作此文章")
	}
	tags := normalizeTags(in.Tags)
	if err := validateArticleInput(in.Title, in.Content, tags); err != nil {
		return nil, err
	}
	tagModels, err := s.tags.EnsureMany(ctx, tags)
	if err != nil {
		return nil, err
	}
	a.Title = strings.TrimSpace(in.Title)
	a.Content = in.Content
	a.Summary = markdownx.Summary(in.Content, 200)
	if err := s.articles.Update(ctx, a, tagModels); err != nil {
		return nil, err
	}
	return s.viewOf(ctx, a, false)
}

func (s *ArticleService) Delete(ctx context.Context, userID, id uint64) error {
	a, err := s.articles.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if a.UserID != userID {
		return apperr.New(apperr.CodeForbidden, "无权操作此文章")
	}
	return s.articles.SoftDelete(ctx, id)
}

func (s *ArticleService) GetByID(ctx context.Context, id uint64, incrementView bool) (*ArticleView, error) {
	a, err := s.articles.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if incrementView {
		_ = s.counter.Inc(ctx, id)
	}
	return s.viewOf(ctx, a, true)
}

func (s *ArticleService) List(ctx context.Context, in ListArticlesInput) (items []ArticleView, total int64, err error) {
	q := repository.ListQuery{Page: in.Page, Size: in.Size, Tag: in.Tag, UserID: in.UserID}
	rows, total, err := s.articles.List(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	out := make([]ArticleView, 0, len(rows))
	for i := range rows {
		v, err := s.viewOf(ctx, &rows[i], true)
		if err != nil {
			return nil, 0, err
		}
		v.Content = ""
		out = append(out, *v)
	}
	return out, total, nil
}

func (s *ArticleService) viewOf(ctx context.Context, a *model.Article, mergeIncrement bool) (*ArticleView, error) {
	tagNames := make([]string, 0, len(a.Tags))
	for _, t := range a.Tags {
		tagNames = append(tagNames, t.Name)
	}
	views := int64(a.ViewCount)
	if mergeIncrement {
		if inc, err := s.counter.GetIncrement(ctx, a.ID); err == nil {
			views += inc
		}
	}
	authorName := ""
	if u, err := s.users.FindByID(ctx, a.UserID); err == nil && u != nil {
		authorName = u.Name
	}
	return &ArticleView{
		ID:         a.ID,
		Title:      a.Title,
		Content:    a.Content,
		Summary:    a.Summary,
		Tags:       tagNames,
		UserID:     a.UserID,
		AuthorName: authorName,
		ViewCount:  views,
		CreatedAt:  a.CreatedAt.Unix(),
		UpdatedAt:  a.UpdatedAt.Unix(),
	}, nil
}
