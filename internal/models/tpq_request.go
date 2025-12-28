// internal/models/tpq_request.go
// Add missing fields if any
package models

import (
	"time"
)

type TPQRequestItem struct {
	RequestID  string `gorm:"primaryKey"`
	ArtifactID string `gorm:"primaryKey"`
	Comment    string
	Artifact   Artifact `gorm:"foreignKey:ArtifactID"`
}

type TPQRequest struct {
	ID          string           `gorm:"primaryKey" json:"id"`
	Status      string           `gorm:"index" json:"status"`
	CreatedAt   time.Time        `json:"created_at"`
	CreatorID   uint             `json:"creator_id"`
	FormedAt    *time.Time       `json:"formed_at,omitempty"`
	CompletedAt *time.Time       `json:"completed_at,omitempty"`
	ModeratorID *uint            `json:"moderator_id,omitempty"`
	Excavation  string           `json:"excavation"`
	Result      *int             `json:"result,omitempty"`
	TPQItems    []TPQRequestItem `gorm:"foreignKey:RequestID" json:"items"`

	ModerationStatus string     `gorm:"default:''" json:"moderation_status"`
	ModeratedAt      *time.Time `json:"moderated_at,omitempty"`
}
