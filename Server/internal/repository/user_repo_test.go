package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/model"
)

func TestUserRepo_CreateAndFind(t *testing.T) {
	db := newTestDB(t)
	repo := NewUserRepo(db)
	ctx := context.Background()

	u := &model.User{Username: "alice", PasswordHash: "h", Name: "爱丽丝"}
	require.NoError(t, repo.Create(ctx, u))
	require.Greater(t, u.ID, uint64(0))

	got, err := repo.FindByUsername(ctx, "alice")
	require.NoError(t, err)
	require.Equal(t, u.ID, got.ID)

	got2, err := repo.FindByID(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, "爱丽丝", got2.Name)
}

func TestUserRepo_Create_DuplicateUsername(t *testing.T) {
	db := newTestDB(t)
	repo := NewUserRepo(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &model.User{Username: "alice", PasswordHash: "h", Name: "x"}))
	err := repo.Create(ctx, &model.User{Username: "alice", PasswordHash: "h", Name: "y"})
	var ae *apperr.AppErr
	require.True(t, errors.As(err, &ae))
	require.Equal(t, apperr.CodeUsernameTaken, ae.Code)
}

func TestUserRepo_FindByUsername_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := NewUserRepo(db)
	_, err := repo.FindByUsername(context.Background(), "ghost")
	var ae *apperr.AppErr
	require.True(t, errors.As(err, &ae))
	require.Equal(t, apperr.CodeNotFound, ae.Code)
}
