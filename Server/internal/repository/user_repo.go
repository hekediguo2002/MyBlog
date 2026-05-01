package repository

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/model"
)

type UserRepo interface {
	Create(ctx context.Context, u *model.User) error
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	FindByID(ctx context.Context, id uint64) (*model.User, error)
}

type userRepo struct{ db *gorm.DB }

func NewUserRepo(db *gorm.DB) UserRepo { return &userRepo{db: db} }

func (r *userRepo) Create(ctx context.Context, u *model.User) error {
	if err := r.db.WithContext(ctx).Create(u).Error; err != nil {
		if isDuplicate(err) {
			return apperr.New(apperr.CodeUsernameTaken, "用户名已被使用")
		}
		return apperr.Wrap(apperr.CodeDBError, "create user", err)
	}
	return nil
}

func (r *userRepo) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.New(apperr.CodeNotFound, "用户不存在")
		}
		return nil, apperr.Wrap(apperr.CodeDBError, "find by username", err)
	}
	return &u, nil
}

func (r *userRepo) FindByID(ctx context.Context, id uint64) (*model.User, error) {
	var u model.User
	if err := r.db.WithContext(ctx).First(&u, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.New(apperr.CodeNotFound, "用户不存在")
		}
		return nil, apperr.Wrap(apperr.CodeDBError, "find by id", err)
	}
	return &u, nil
}

func isDuplicate(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique") || strings.Contains(msg, "constraint failed")
}
