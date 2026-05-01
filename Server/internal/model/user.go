package model

import "time"

type User struct {
	ID           uint64    `gorm:"primaryKey;column:id"`
	Username     string    `gorm:"size:32;uniqueIndex:uk_username;column:username"`
	PasswordHash string    `gorm:"size:60;column:password_hash"`
	Name         string    `gorm:"size:64;column:name"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (User) TableName() string { return "users" }
