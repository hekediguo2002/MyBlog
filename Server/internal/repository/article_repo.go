package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/model"
)

type ListQuery struct {
	Page   int
	Size   int
	Tag    string
	UserID uint64
}

type ArticleRepo interface {
	Create(ctx context.Context, a *model.Article, tags []model.Tag) error
	Update(ctx context.Context, a *model.Article, tags []model.Tag) error
	SoftDelete(ctx context.Context, id uint64) error
	FindByID(ctx context.Context, id uint64) (*model.Article, error)
	List(ctx context.Context, q ListQuery) ([]model.Article, int64, error)
	IncrementViewCount(ctx context.Context, id uint64, delta int64) error
}

type articleRepo struct{ db *gorm.DB }

func NewArticleRepo(db *gorm.DB) ArticleRepo { return &articleRepo{db: db} }

func (r *articleRepo) Create(ctx context.Context, a *model.Article, tags []model.Tag) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(a).Error; err != nil {
			return apperr.Wrap(apperr.CodeDBError, "create article", err)
		}
		if len(tags) > 0 {
			if err := tx.Model(a).Association("Tags").Replace(tags); err != nil {
				return apperr.Wrap(apperr.CodeDBError, "set tags", err)
			}
		}
		return nil
	})
}

func (r *articleRepo) Update(ctx context.Context, a *model.Article, tags []model.Tag) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Article{}).
			Where("id = ?", a.ID).
			Updates(map[string]any{"title": a.Title, "content": a.Content, "summary": a.Summary})
		if res.Error != nil {
			return apperr.Wrap(apperr.CodeDBError, "update article", res.Error)
		}
		if res.RowsAffected == 0 {
			return apperr.New(apperr.CodeNotFound, "文章不存在")
		}
		if err := tx.Model(a).Association("Tags").Replace(tags); err != nil {
			return apperr.Wrap(apperr.CodeDBError, "replace tags", err)
		}
		return nil
	})
}

func (r *articleRepo) SoftDelete(ctx context.Context, id uint64) error {
	res := r.db.WithContext(ctx).Delete(&model.Article{}, id)
	if res.Error != nil {
		return apperr.Wrap(apperr.CodeDBError, "delete article", res.Error)
	}
	if res.RowsAffected == 0 {
		return apperr.New(apperr.CodeNotFound, "文章不存在")
	}
	return nil
}

func (r *articleRepo) FindByID(ctx context.Context, id uint64) (*model.Article, error) {
	var a model.Article
	err := r.db.WithContext(ctx).
		Preload("Tags").
		Preload("Author").
		First(&a, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.New(apperr.CodeNotFound, "文章不存在")
		}
		return nil, apperr.Wrap(apperr.CodeDBError, "find article", err)
	}
	return &a, nil
}

func (r *articleRepo) List(ctx context.Context, q ListQuery) ([]model.Article, int64, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Size < 1 || q.Size > 50 {
		q.Size = 10
	}
	tx := r.db.WithContext(ctx).Model(&model.Article{}).Where("status = ?", 1)
	if q.UserID > 0 {
		tx = tx.Where("user_id = ?", q.UserID)
	}
	if q.Tag != "" {
		sub := r.db.Table("article_tags").
			Select("article_tags.article_id").
			Joins("JOIN tags ON tags.id = article_tags.tag_id").
			Where("tags.name = ?", q.Tag)
		tx = tx.Where("id IN (?)", sub)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, apperr.Wrap(apperr.CodeDBError, "count list", err)
	}
	var rows []model.Article
	err := tx.Preload("Tags").Preload("Author").
		Order("created_at DESC").
		Limit(q.Size).Offset((q.Page - 1) * q.Size).
		Find(&rows).Error
	if err != nil {
		return nil, 0, apperr.Wrap(apperr.CodeDBError, "list articles", err)
	}
	return rows, total, nil
}

func (r *articleRepo) IncrementViewCount(ctx context.Context, id uint64, delta int64) error {
	if delta == 0 {
		return nil
	}
	res := r.db.WithContext(ctx).Model(&model.Article{}).
		Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + ?", delta))
	if res.Error != nil {
		return apperr.Wrap(apperr.CodeDBError, "incr view count", res.Error)
	}
	return nil
}
