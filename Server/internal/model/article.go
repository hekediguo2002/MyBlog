package model

import (
	"time"

	"gorm.io/gorm"
)

type Article struct {
	ID        uint64         `gorm:"primaryKey;column:id"`
	UserID    uint64         `gorm:"index:idx_user_id;column:user_id"`
	Title     string         `gorm:"size:200;column:title"`
	Content   string         `gorm:"type:text;column:content"`
	Summary   string         `gorm:"size:500;column:summary"`
	ViewCount uint64         `gorm:"column:view_count;default:0"`
	Status    int8           `gorm:"column:status;default:1"`
	CreatedAt time.Time      `gorm:"column:created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`

	Tags   []Tag `gorm:"many2many:article_tags;joinForeignKey:article_id;joinReferences:tag_id"`
	Author User  `gorm:"foreignKey:UserID;references:ID"`
}

func (Article) TableName() string { return "articles" }
