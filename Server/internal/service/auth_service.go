package service

import (
	"context"
	"regexp"
	"unicode/utf8"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/model"
	"github.com/wjr/blog/server/internal/pkg/password"
	"github.com/wjr/blog/server/internal/repository"
)

type RegisterInput struct {
	Username string
	Password string
	Name     string
}

type AuthService interface {
	Register(ctx context.Context, in RegisterInput) (*model.User, error)
	Login(ctx context.Context, username, plain string) (*model.User, error)
	GetByID(ctx context.Context, id uint64) (*model.User, error)
}

type authService struct{ users repository.UserRepo }

func NewAuthService(u repository.UserRepo) AuthService { return &authService{users: u} }

var reUsername = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func validatePassword(p string) error {
	if len(p) < 8 || len(p) > 64 {
		return apperr.New(apperr.CodeInvalidParam, "密码长度需 8–64")
	}
	hasLetter, hasDigit := false, false
	for _, ch := range p {
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z':
			hasLetter = true
		case ch >= '0' && ch <= '9':
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return apperr.New(apperr.CodeInvalidParam, "密码需包含字母与数字")
	}
	return nil
}

func validateRegister(in RegisterInput) error {
	if l := len(in.Username); l < 4 || l > 32 {
		return apperr.New(apperr.CodeInvalidParam, "用户名长度需 4–32")
	}
	if !reUsername.MatchString(in.Username) {
		return apperr.New(apperr.CodeInvalidParam, "用户名仅允许字母数字下划线")
	}
	if err := validatePassword(in.Password); err != nil {
		return err
	}
	if l := utf8.RuneCountInString(in.Name); l < 1 || l > 64 {
		return apperr.New(apperr.CodeInvalidParam, "昵称长度需 1–64")
	}
	return nil
}

func (s *authService) Register(ctx context.Context, in RegisterInput) (*model.User, error) {
	if err := validateRegister(in); err != nil {
		return nil, err
	}
	hash, err := password.Hash(in.Password)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeUnknown, "hash", err)
	}
	u := &model.User{Username: in.Username, PasswordHash: hash, Name: in.Name}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *authService) Login(ctx context.Context, username, plain string) (*model.User, error) {
	u, err := s.users.FindByUsername(ctx, username)
	if err != nil {
		return nil, apperr.New(apperr.CodeBadCredential, "账号或密码错误")
	}
	if !password.Compare(u.PasswordHash, plain) {
		return nil, apperr.New(apperr.CodeBadCredential, "账号或密码错误")
	}
	return u, nil
}

func (s *authService) GetByID(ctx context.Context, id uint64) (*model.User, error) {
	return s.users.FindByID(ctx, id)
}
