package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/model"
	"github.com/wjr/blog/server/internal/pkg/password"
)

type fakeUserRepo struct {
	byUsername map[string]*model.User
	byID       map[uint64]*model.User
	nextID     uint64
	createErr  error
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{byUsername: map[string]*model.User{}, byID: map[uint64]*model.User{}, nextID: 1}
}
func (f *fakeUserRepo) Create(_ context.Context, u *model.User) error {
	if f.createErr != nil {
		return f.createErr
	}
	if _, ok := f.byUsername[u.Username]; ok {
		return apperr.New(apperr.CodeUsernameTaken, "dup")
	}
	u.ID = f.nextID
	f.nextID++
	f.byUsername[u.Username] = u
	f.byID[u.ID] = u
	return nil
}
func (f *fakeUserRepo) FindByUsername(_ context.Context, n string) (*model.User, error) {
	u, ok := f.byUsername[n]
	if !ok {
		return nil, apperr.New(apperr.CodeNotFound, "")
	}
	return u, nil
}
func (f *fakeUserRepo) FindByID(_ context.Context, id uint64) (*model.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return nil, apperr.New(apperr.CodeNotFound, "")
	}
	return u, nil
}

func TestAuthService_Register_Success(t *testing.T) {
	repo := newFakeUserRepo()
	svc := NewAuthService(repo)
	u, err := svc.Register(context.Background(), RegisterInput{Username: "alice", Password: "Password123", Name: "A"})
	require.NoError(t, err)
	require.Equal(t, uint64(1), u.ID)
	require.True(t, password.Compare(repo.byID[1].PasswordHash, "Password123"))
}

func TestAuthService_Register_DuplicateUsername(t *testing.T) {
	repo := newFakeUserRepo()
	svc := NewAuthService(repo)
	_, _ = svc.Register(context.Background(), RegisterInput{Username: "alice", Password: "Password123", Name: "A"})
	_, err := svc.Register(context.Background(), RegisterInput{Username: "alice", Password: "Password123", Name: "B"})
	var ae *apperr.AppErr
	require.True(t, errors.As(err, &ae))
	require.Equal(t, apperr.CodeUsernameTaken, ae.Code)
}

func TestAuthService_Register_InvalidInput(t *testing.T) {
	repo := newFakeUserRepo()
	svc := NewAuthService(repo)
	cases := []RegisterInput{
		{Username: "abc", Password: "Password123", Name: "A"},          // 用户名 < 4
		{Username: "ok", Password: "short", Name: "A"},                  // 密码 < 8
		{Username: "ok", Password: "alphabet", Name: "A"},               // 密码无数字
		{Username: "ok", Password: "12345678", Name: "A"},               // 密码无字母
		{Username: "bad-name!", Password: "Password123", Name: "A"},     // 用户名含非法字符
		{Username: "alice", Password: "Password123", Name: ""},          // 昵称空
	}
	for _, in := range cases {
		_, err := svc.Register(context.Background(), in)
		var ae *apperr.AppErr
		require.True(t, errors.As(err, &ae), "expected AppErr for %+v", in)
		require.Equal(t, apperr.CodeInvalidParam, ae.Code, "case %+v", in)
	}
}

func TestAuthService_Login_Success(t *testing.T) {
	repo := newFakeUserRepo()
	svc := NewAuthService(repo)
	_, _ = svc.Register(context.Background(), RegisterInput{Username: "alice", Password: "Password123", Name: "A"})
	u, err := svc.Login(context.Background(), "alice", "Password123")
	require.NoError(t, err)
	require.Equal(t, uint64(1), u.ID)
}

func TestAuthService_Login_BadCredential(t *testing.T) {
	repo := newFakeUserRepo()
	svc := NewAuthService(repo)
	_, _ = svc.Register(context.Background(), RegisterInput{Username: "alice", Password: "Password123", Name: "A"})

	_, err := svc.Login(context.Background(), "alice", "wrong")
	var ae *apperr.AppErr
	require.True(t, errors.As(err, &ae))
	require.Equal(t, apperr.CodeBadCredential, ae.Code)

	_, err = svc.Login(context.Background(), "ghost", "anything")
	require.True(t, errors.As(err, &ae))
	require.Equal(t, apperr.CodeBadCredential, ae.Code)
}
