package data

import (
	"time"
)

const (
	DelStateNo  int32 = 0
	DelStateYes int32 = 1
)

type BaseModel struct {
	Id        int64      `gorm:"column:id;primaryKey;autoIncrement"`
	DelState  int32      `gorm:"column:del_state;default:0"`
	DeletedAt *time.Time `gorm:"column:deleted_at"`
	Version   int64      `gorm:"column:version;default:0"`
	CreatedAt time.Time  `gorm:"column:created_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at"`
}
