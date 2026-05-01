package repository

import (
	"context"
	"errors"
	"strconv"

	"github.com/redis/go-redis/v9"

	"github.com/wjr/blog/server/internal/apperr"
)

type CounterRepo interface {
	Inc(ctx context.Context, articleID uint64) error
	GetIncrement(ctx context.Context, articleID uint64) (int64, error)
	DirtyMembers(ctx context.Context) ([]uint64, error)
	DrainIncrement(ctx context.Context, articleID uint64) (int64, error)
	Ack(ctx context.Context, ids []uint64) error
	Restore(ctx context.Context, articleID uint64, delta int64) error
}

type counterRepo struct{ rdb *redis.Client }

func NewCounterRepo(rdb *redis.Client) CounterRepo { return &counterRepo{rdb: rdb} }

func keyView(id uint64) string  { return "view:" + strconv.FormatUint(id, 10) }
func keyDirty() string          { return "view:dirty" }

func (r *counterRepo) Inc(ctx context.Context, id uint64) error {
	pipe := r.rdb.Pipeline()
	pipe.Incr(ctx, keyView(id))
	pipe.SAdd(ctx, keyDirty(), id)
	if _, err := pipe.Exec(ctx); err != nil {
		return apperr.Wrap(apperr.CodeRedisError, "view inc", err)
	}
	return nil
}

func (r *counterRepo) GetIncrement(ctx context.Context, id uint64) (int64, error) {
	v, err := r.rdb.Get(ctx, keyView(id)).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, apperr.Wrap(apperr.CodeRedisError, "view get", err)
	}
	return v, nil
}

func (r *counterRepo) DirtyMembers(ctx context.Context) ([]uint64, error) {
	out, err := r.rdb.SMembers(ctx, keyDirty()).Result()
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeRedisError, "smembers dirty", err)
	}
	ids := make([]uint64, 0, len(out))
	for _, s := range out {
		if id, err := strconv.ParseUint(s, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (r *counterRepo) DrainIncrement(ctx context.Context, id uint64) (int64, error) {
	v, err := r.rdb.GetSet(ctx, keyView(id), 0).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, apperr.Wrap(apperr.CodeRedisError, "view drain", err)
	}
	return v, nil
}

func (r *counterRepo) Ack(ctx context.Context, ids []uint64) error {
	if len(ids) == 0 {
		return nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	if err := r.rdb.SRem(ctx, keyDirty(), args...).Err(); err != nil {
		return apperr.Wrap(apperr.CodeRedisError, "srem dirty", err)
	}
	return nil
}

func (r *counterRepo) Restore(ctx context.Context, id uint64, delta int64) error {
	if delta <= 0 {
		return nil
	}
	if err := r.rdb.IncrBy(ctx, keyView(id), delta).Err(); err != nil {
		return apperr.Wrap(apperr.CodeRedisError, "view restore", err)
	}
	return nil
}
