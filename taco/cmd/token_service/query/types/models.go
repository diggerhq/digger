package types

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Token represents an API token for authentication
type Token struct {
	ID         string    `gorm:"type:varchar(36);primaryKey"`
	UserID     string    `gorm:"type:varchar(255);index;not null"` // Flexible for external user IDs
	OrgID      string    `gorm:"type:varchar(255);index;not null"` // Flexible for external org IDs
	Token      string    `gorm:"type:varchar(255);uniqueIndex;not null"`
	Name       string    `gorm:"type:varchar(255)"`
	Status     string    `gorm:"type:varchar(20);default:active"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
}

func (t *Token) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

func (Token) TableName() string { return "tokens" }

// TokenModels contains all models for token service migrations
var TokenModels = []any{
	&Token{},
}

