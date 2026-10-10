package model

import (
	"time"

	"gorm.io/gorm"
)

// Timestamps kolom audit standar beserta dukungan soft delete (deleted_at).
type Timestamps struct {
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index" swaggertype:"string"`
}
