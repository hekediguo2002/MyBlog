package service

import (
	"context"

	"github.com/wjr/blog/server/internal/repository"
)

type userView struct {
	ID        uint64 `json:"id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
}

type AdminService interface {
	ListUsers(ctx context.Context) ([]userView, error)
	DeleteUser(ctx context.Context, id uint64) error
}

type adminService struct {
	users    repository.UserRepo
	articles repository.ArticleRepo
}

func NewAdminService(users repository.UserRepo, articles repository.ArticleRepo) AdminService {
	return &adminService{users: users, articles: articles}
}

func (s *adminService) ListUsers(ctx context.Context) ([]userView, error) {
	users, err := s.users.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]userView, len(users))
	for i, u := range users {
		out[i] = userView{
			ID:        u.ID,
			Username:  u.Username,
			Name:      u.Name,
			CreatedAt: u.CreatedAt.Unix(),
		}
	}
	return out, nil
}

func (s *adminService) DeleteUser(ctx context.Context, id uint64) error {
	if err := s.articles.HardDeleteByUserID(ctx, id); err != nil {
		return err
	}
	return s.users.HardDelete(ctx, id)
}
