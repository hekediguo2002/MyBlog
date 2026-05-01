package model

type Tag struct {
	ID   uint64 `gorm:"primaryKey;column:id"`
	Name string `gorm:"size:32;uniqueIndex:uk_name;column:name"`
}

func (Tag) TableName() string { return "tags" }
