package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/model"
)

type TagWithCount struct {
	ID           uint64 `json:"id"`
	Name         string `json:"name"`
	ArticleCount int    `json:"article_count"`
}

type TagRepo interface {
	EnsureMany(ctx context.Context, names []string) ([]model.Tag, error)
	ListWithCount(ctx context.Context) ([]TagWithCount, error)
}

type tagRepo struct{ db *gorm.DB }

func NewTagRepo(db *gorm.DB) TagRepo { return &tagRepo{db: db} }

func (r *tagRepo) EnsureMany(ctx context.Context, names []string) ([]model.Tag, error) {
	uniq := dedupNonEmpty(names)
	if len(uniq) == 0 {
		return nil, nil
	}
	rows := make([]model.Tag, len(uniq))
	for i, n := range uniq {
		rows[i] = model.Tag{Name: n}
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoNothing: true}).
		Create(&rows).Error; err != nil {
		return nil, apperr.Wrap(apperr.CodeDBError, "upsert tags", err)
	}
	var out []model.Tag
	if err := r.db.WithContext(ctx).Where("name IN ?", uniq).Find(&out).Error; err != nil {
		return nil, apperr.Wrap(apperr.CodeDBError, "load tags", err)
	}
	return out, nil
}

func (r *tagRepo) ListWithCount(ctx context.Context) ([]TagWithCount, error) {
	var rows []TagWithCount
	err := r.db.WithContext(ctx).
		Table("tags").
		Select("tags.id, tags.name, COUNT(article_tags.article_id) AS article_count").
		Joins("LEFT JOIN article_tags ON article_tags.tag_id = tags.id").
		Group("tags.id, tags.name").
		Order("article_count DESC, tags.name ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeDBError, "list tags", err)
	}
	return rows, nil
}

func dedupNonEmpty(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
