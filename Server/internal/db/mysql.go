package db

import (
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlog "gorm.io/gorm/logger"
)

type Options struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	SlowThresholdMS int
}

func DefaultOptions(dsn string) Options {
	return Options{
		DSN:             dsn,
		MaxOpenConns:    50,
		MaxIdleConns:    10,
		ConnMaxLifetime: 30 * time.Minute,
		SlowThresholdMS: 200,
	}
}

func Open(opt Options) (*gorm.DB, error) {
	gdb, err := gorm.Open(mysql.Open(opt.DSN), &gorm.Config{
		Logger: gormlog.Default.LogMode(gormlog.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("gorm open: %w", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(opt.MaxOpenConns)
	sqlDB.SetMaxIdleConns(opt.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(opt.ConnMaxLifetime)
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return gdb, nil
}
