package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/example/task-management-api/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TeamRepository struct{ pool *pgxpool.Pool }

func NewTeamRepository(pool *pgxpool.Pool) *TeamRepository { return &TeamRepository{pool: pool} }

// Create creates a team and enrolls its authenticated creator atomically.
func (r *TeamRepository) Create(ctx context.Context, creatorID uuid.UUID, name string) (domain.Team, error) {
	const query = `WITH created_team AS (
		INSERT INTO teams (name) VALUES ($1)
		RETURNING id, public_id, name, created_at, updated_at
	), creator_membership AS (
		INSERT INTO team_members (team_id, user_id)
		SELECT created_team.id, users.id
		FROM created_team
		JOIN users ON users.public_id = $2
	)
	SELECT public_id, name, created_at, updated_at FROM created_team`

	var team domain.Team
	if err := r.pool.QueryRow(ctx, query, name, creatorID).Scan(&team.ID, &team.Name, &team.CreatedAt, &team.UpdatedAt); err != nil {
		return domain.Team{}, fmt.Errorf("create team: %w", err)
	}
	return team, nil
}

func (r *TeamRepository) ListByMember(ctx context.Context, userID uuid.UUID) ([]domain.Team, error) {
	const query = `SELECT teams.public_id, teams.name, teams.created_at, teams.updated_at
		FROM teams
		JOIN team_members ON team_members.team_id = teams.id
		JOIN users ON users.id = team_members.user_id
		WHERE users.public_id = $1
		ORDER BY teams.created_at ASC, teams.id ASC`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list teams: %w", err)
	}
	defer rows.Close()

	teams := make([]domain.Team, 0)
	for rows.Next() {
		var team domain.Team
		if err := rows.Scan(&team.ID, &team.Name, &team.CreatedAt, &team.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan team: %w", err)
		}
		teams = append(teams, team)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate teams: %w", err)
	}
	return teams, nil
}

func (r *TeamRepository) ListMembers(ctx context.Context, requesterID, teamID uuid.UUID) ([]domain.TeamMember, error) {
	teamInternalID, err := r.authorizedTeamID(ctx, r.pool, requesterID, teamID)
	if err != nil {
		return nil, err
	}
	const query = `SELECT users.public_id, users.name, users.email, team_members.created_at
		FROM team_members
		JOIN users ON users.id = team_members.user_id
		WHERE team_members.team_id = $1
		ORDER BY team_members.created_at ASC, users.id ASC`
	rows, err := r.pool.Query(ctx, query, teamInternalID)
	if err != nil {
		return nil, fmt.Errorf("list team members: %w", err)
	}
	defer rows.Close()

	members := make([]domain.TeamMember, 0)
	for rows.Next() {
		var member domain.TeamMember
		if err := rows.Scan(&member.ID, &member.Name, &member.Email, &member.JoinedAt); err != nil {
			return nil, fmt.Errorf("scan team member: %w", err)
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate team members: %w", err)
	}
	return members, nil
}

func (r *TeamRepository) AddMember(ctx context.Context, requesterID, teamID, userID uuid.UUID) (err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin add member transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	teamInternalID, err := r.authorizedTeamID(ctx, tx, requesterID, teamID)
	if err != nil {
		return err
	}
	userInternalID, err := lookupUserID(ctx, tx, userID)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO team_members (team_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, teamInternalID, userInternalID); err != nil {
		return fmt.Errorf("add team member: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit add member transaction: %w", err)
	}
	return nil
}

func (r *TeamRepository) RemoveMember(ctx context.Context, requesterID, teamID, userID uuid.UUID) (err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin remove member transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	teamInternalID, err := r.authorizedTeamID(ctx, tx, requesterID, teamID)
	if err != nil {
		return err
	}
	userInternalID, err := lookupUserID(ctx, tx, userID)
	if err != nil {
		return err
	}
	command, err := tx.Exec(ctx, `DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`, teamInternalID, userInternalID)
	if err != nil {
		return fmt.Errorf("remove team member: %w", err)
	}
	if command.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit remove member transaction: %w", err)
	}
	return nil
}

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *TeamRepository) authorizedTeamID(ctx context.Context, db queryRower, requesterID, teamID uuid.UUID) (int64, error) {
	internalTeamID, err := lookupTeamID(ctx, db, teamID)
	if err != nil {
		return 0, err
	}
	var isMember bool
	if err := db.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM team_members
		JOIN users ON users.id = team_members.user_id
		WHERE team_members.team_id = $1 AND users.public_id = $2
	)`, internalTeamID, requesterID).Scan(&isMember); err != nil {
		return 0, fmt.Errorf("check team membership: %w", err)
	}
	if !isMember {
		return 0, domain.ErrForbidden
	}
	return internalTeamID, nil
}

func lookupUserID(ctx context.Context, db queryRower, publicID uuid.UUID) (int64, error) {
	var internalID int64
	if err := db.QueryRow(ctx, `SELECT id FROM users WHERE public_id = $1`, publicID).Scan(&internalID); err != nil {
		return 0, mapTeamLookupError(err)
	}
	return internalID, nil
}

func lookupTeamID(ctx context.Context, db queryRower, publicID uuid.UUID) (int64, error) {
	var internalID int64
	if err := db.QueryRow(ctx, `SELECT id FROM teams WHERE public_id = $1`, publicID).Scan(&internalID); err != nil {
		return 0, mapTeamLookupError(err)
	}
	return internalID, nil
}

func mapTeamLookupError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return fmt.Errorf("lookup team membership target: %w", err)
}
