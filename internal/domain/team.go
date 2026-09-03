package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Team struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TeamMember struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Email    string    `json:"email"`
	JoinedAt time.Time `json:"joined_at"`
}

type TeamRepository interface {
	Create(ctx context.Context, creatorID uuid.UUID, name string) (Team, error)
	ListByMember(ctx context.Context, userID uuid.UUID) ([]Team, error)
	ListMembers(ctx context.Context, requesterID, teamID uuid.UUID) ([]TeamMember, error)
	AddMember(ctx context.Context, requesterID, teamID, userID uuid.UUID) error
	RemoveMember(ctx context.Context, requesterID, teamID, userID uuid.UUID) error
}
