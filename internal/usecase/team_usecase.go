package usecase

import (
	"context"
	"strings"

	"github.com/example/task-management-api/internal/domain"
	"github.com/google/uuid"
)

type TeamUsecase struct{ teams domain.TeamRepository }

func NewTeamUsecase(teams domain.TeamRepository) *TeamUsecase { return &TeamUsecase{teams: teams} }

func (u *TeamUsecase) Create(ctx context.Context, creatorID uuid.UUID, name string) (domain.Team, error) {
	name = strings.TrimSpace(name)
	if creatorID == uuid.Nil || name == "" || len(name) > 100 {
		return domain.Team{}, domain.ErrInvalidInput
	}
	return u.teams.Create(ctx, creatorID, name)
}

func (u *TeamUsecase) ListMyTeams(ctx context.Context, userID uuid.UUID) ([]domain.Team, error) {
	if userID == uuid.Nil {
		return nil, domain.ErrInvalidInput
	}
	return u.teams.ListByMember(ctx, userID)
}

func (u *TeamUsecase) ListMembers(ctx context.Context, requesterID, teamID uuid.UUID) ([]domain.TeamMember, error) {
	if requesterID == uuid.Nil || teamID == uuid.Nil {
		return nil, domain.ErrInvalidInput
	}
	return u.teams.ListMembers(ctx, requesterID, teamID)
}

func (u *TeamUsecase) AddMember(ctx context.Context, requesterID, teamID, userID uuid.UUID) error {
	if requesterID == uuid.Nil || teamID == uuid.Nil || userID == uuid.Nil {
		return domain.ErrInvalidInput
	}
	return u.teams.AddMember(ctx, requesterID, teamID, userID)
}

func (u *TeamUsecase) RemoveMember(ctx context.Context, requesterID, teamID, userID uuid.UUID) error {
	if requesterID == uuid.Nil || teamID == uuid.Nil || userID == uuid.Nil {
		return domain.ErrInvalidInput
	}
	return u.teams.RemoveMember(ctx, requesterID, teamID, userID)
}
