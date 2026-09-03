package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/example/task-management-api/internal/domain"
	"github.com/example/task-management-api/internal/usecase"
	"github.com/google/uuid"
)

func TestTeamUsecase(t *testing.T) {
	userID := uuid.New()
	tests := []struct {
		name      string
		operation string
		userID    uuid.UUID
		teamName  string
		wantErr   error
	}{
		{name: "creates normalized team", operation: "create", userID: userID, teamName: "  Backend Team  "},
		{name: "rejects blank team name", operation: "create", userID: userID, teamName: " ", wantErr: domain.ErrInvalidInput},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &fakeTeamRepository{}
			service := usecase.NewTeamUsecase(repository)

			if test.operation == "create" {
				_, err := service.Create(context.Background(), test.userID, test.teamName)
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("Create() error = %v, want %v", err, test.wantErr)
				}
				if test.wantErr == nil && (repository.createName != "Backend Team" || repository.creatorID != userID) {
					t.Fatalf("unexpected create arguments: name=%q creator=%s", repository.createName, repository.creatorID)
				}
				return
			}

		})
	}
}

type fakeTeamRepository struct {
	creatorID  uuid.UUID
	createName string
}

func (r *fakeTeamRepository) Create(_ context.Context, creatorID uuid.UUID, name string) (domain.Team, error) {
	r.creatorID, r.createName = creatorID, name
	return domain.Team{ID: uuid.New(), Name: name}, nil
}
func (r *fakeTeamRepository) ListByMember(context.Context, uuid.UUID) ([]domain.Team, error) {
	return nil, nil
}
func (r *fakeTeamRepository) ListMembers(context.Context, uuid.UUID, uuid.UUID) ([]domain.TeamMember, error) {
	return nil, nil
}
func (r *fakeTeamRepository) AddMember(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}
func (r *fakeTeamRepository) RemoveMember(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}
