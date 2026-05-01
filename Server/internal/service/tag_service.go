package service

import (
	"context"
	"sort"

	"github.com/wjr/blog/server/internal/repository"
)

type TagView struct {
	Name         string `json:"name"`
	ArticleCount int64  `json:"article_count"`
}

type TagService struct {
	tags repository.TagRepo
}

func NewTagService(tags repository.TagRepo) *TagService {
	return &TagService{tags: tags}
}

func (s *TagService) List(ctx context.Context) ([]TagView, error) {
	rows, err := s.tags.ListWithCount(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]TagView, 0, len(rows))
	for _, r := range rows {
		out = append(out, TagView{Name: r.Name, ArticleCount: int64(r.ArticleCount)})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ArticleCount != out[j].ArticleCount {
			return out[i].ArticleCount > out[j].ArticleCount
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}
