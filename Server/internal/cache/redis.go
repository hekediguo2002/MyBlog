package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Options struct {
	Addr         string
	DB           int
	PoolSize     int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func Default(addr string, db int) Options {
	return Options{
		Addr:         addr,
		DB:           db,
		PoolSize:     20,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
}

func Open(opt Options) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         opt.Addr,
		DB:           opt.DB,
		PoolSize:     opt.PoolSize,
		DialTimeout:  opt.DialTimeout,
		ReadTimeout:  opt.ReadTimeout,
		WriteTimeout: opt.WriteTimeout,
	})
	ctx, cancel := context.WithTimeout(context.Background(), opt.DialTimeout+opt.ReadTimeout)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return rdb, nil
}
