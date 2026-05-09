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
	ListAll(ctx context.Context) ([]model.User, error)
	HardDelete(ctx context.Context, id uint64) error
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

func (r *userRepo) ListAll(ctx context.Context) ([]model.User, error) {
	var users []model.User
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&users).Error; err != nil {
		return nil, apperr.Wrap(apperr.CodeDBError, "list users", err)
	}
	return users, nil
}

func (r *userRepo) HardDelete(ctx context.Context, id uint64) error {
	res := r.db.WithContext(ctx).Unscoped().Delete(&model.User{}, id)
	if res.Error != nil {
		return apperr.Wrap(apperr.CodeDBError, "hard delete user", res.Error)
	}
	if res.RowsAffected == 0 {
		return apperr.New(apperr.CodeNotFound, "用户不存在")
	}
	return nil
}

func isDuplicate(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique") || strings.Contains(msg, "constraint failed")
}
