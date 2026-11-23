package postgres

import (
	"context"
	"errors"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/invitations"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type InvitationsRepo struct {
	q *sqlc.Queries
}

func NewInvitationsRepo(pool *pgxpool.Pool) *InvitationsRepo {
	return &InvitationsRepo{q: sqlc.New(pool)}
}

// targetColumns splits a target into the nullable columns that reference it.
func targetColumns(t invitations.Target) (org, team, contest *uuid.UUID) {
	id := t.ID
	switch t.Kind {
	case invitations.KindOrganization:
		return &id, nil, nil
	case invitations.KindTeam:
		return nil, &id, nil
	case invitations.KindContest:
		return nil, nil, &id
	}
	return nil, nil, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (r *InvitationsRepo) OrganizationID(ctx context.Context, login string) (uuid.UUID, error) {
	org, err := r.q.GetOrganizationByLogin(ctx, login)
	if err != nil {
		return uuid.Nil, dbError("get organization", err)
	}
	return org.ID, nil
}

func (r *InvitationsRepo) ContestID(ctx context.Context, orgLogin, contestLogin string) (uuid.UUID, error) {
	id, err := r.q.GetContestIDByLogins(ctx, sqlc.GetContestIDByLoginsParams{OrgLogin: orgLogin, ContestLogin: contestLogin})
	return id, dbError("get contest", err)
}

func (r *InvitationsRepo) CreateInvitation(ctx context.Context, input invitations.NewInvitation) (uuid.UUID, error) {
	org, _, contest := targetColumns(input.Target)
	id, err := r.q.CreateInvitation(ctx, sqlc.CreateInvitationParams{
		OrganizationID: org,
		ContestID:      contest,
		Role:           input.Role,
		UserID:         &input.UserID,
		InviterID:      input.InviterID,
	})
	return id, dbError("create invitation", err)
}

func (r *InvitationsRepo) GetInvitation(ctx context.Context, id uuid.UUID) (invitations.Invitation, error) {
	row, err := r.q.GetInvitation(ctx, id)
	if err != nil {
		return invitations.Invitation{}, dbError("get invitation", err)
	}
	target := invitations.Target{
		OrganizationID:    row.TargetOrganizationID,
		OrganizationLogin: row.OrganizationLogin,
		OrganizationName:  row.OrganizationName,
	}
	switch {
	case row.OrganizationID != nil:
		target.Kind, target.ID = invitations.KindOrganization, *row.OrganizationID
	case row.TeamID != nil:
		target.Kind, target.ID = invitations.KindTeam, *row.TeamID
	case row.ContestID != nil:
		target.Kind, target.ID = invitations.KindContest, *row.ContestID
		target.ContestLogin, target.ContestTitle = deref(row.ContestLogin), deref(row.ContestTitle)
	}
	inv := invitations.Invitation{
		ID:              row.ID,
		Target:          target,
		Username:        deref(row.Username),
		InviterUsername: row.InviterUsername,
		Role:            row.Role,
		CreatedAt:       row.CreatedAt,
	}
	if row.UserID != nil {
		inv.UserID = *row.UserID
	}
	return inv, nil
}

func (r *InvitationsRepo) ListInvitations(ctx context.Context, target invitations.Target) ([]invitations.Invitation, error) {
	org, _, contest := targetColumns(target)
	rows, err := r.q.ListInvitations(ctx, sqlc.ListInvitationsParams{OrganizationID: org, ContestID: contest})
	if err != nil {
		return nil, dbError("list invitations", err)
	}
	items := make([]invitations.Invitation, len(rows))
	for i, row := range rows {
		items[i] = invitations.Invitation{
			ID:              row.ID,
			Target:          target,
			UserID:          *row.UserID,
			Username:        row.Username,
			InviterUsername: row.InviterUsername,
			Role:            row.Role,
			CreatedAt:       row.CreatedAt,
		}
	}
	return items, nil
}

func (r *InvitationsRepo) DeleteInvitation(ctx context.Context, id uuid.UUID) error {
	n, err := r.q.DeleteInvitation(ctx, id)
	if err != nil {
		return dbError("delete invitation", err)
	}
	if n == 0 {
		return pkg.Wrap(pkg.ErrRequestResolved, nil, "the invitation is no longer pending")
	}
	return nil
}

func (r *InvitationsRepo) JoinLink(ctx context.Context, target invitations.Target) (*string, error) {
	_, team, contest := targetColumns(target)
	token, err := r.q.GetJoinLink(ctx, sqlc.GetJoinLinkParams{TeamID: team, ContestID: contest})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return token, dbError("get join link", err)
}

func (r *InvitationsRepo) SetJoinLink(ctx context.Context, target invitations.Target, token string, inviterID uuid.UUID) error {
	_, team, contest := targetColumns(target)
	role := string(accessmodel.TeamMember)
	if target.Kind == invitations.KindContest {
		role = string(accessmodel.ContestParticipant)
	}
	return dbError("set join link", r.q.SetJoinLink(ctx, sqlc.SetJoinLinkParams{TeamID: team, ContestID: contest, Role: role, Token: token, InviterID: inviterID}))
}

func (r *InvitationsRepo) DeleteJoinLink(ctx context.Context, target invitations.Target) error {
	_, team, contest := targetColumns(target)
	return dbError("delete join link", r.q.DeleteJoinLink(ctx, sqlc.DeleteJoinLinkParams{TeamID: team, ContestID: contest}))
}

func (r *InvitationsRepo) JoinLinkTarget(ctx context.Context, token string) (invitations.Target, error) {
	row, err := r.q.GetJoinLinkByToken(ctx, token)
	if err != nil {
		return invitations.Target{}, dbError("get join link", err)
	}
	target := invitations.Target{
		OrganizationID:    row.OrganizationID,
		OrganizationLogin: row.OrganizationLogin,
		OrganizationName:  row.OrganizationName,
		TeamName:          deref(row.TeamName),
		ContestLogin:      deref(row.ContestLogin),
		ContestTitle:      deref(row.ContestTitle),
	}
	if row.TeamID != nil {
		target.Kind, target.ID = invitations.KindTeam, *row.TeamID
	} else if row.ContestID != nil {
		target.Kind, target.ID = invitations.KindContest, *row.ContestID
	}
	return target, nil
}

func (r *InvitationsRepo) OrganizationRole(ctx context.Context, orgID, userID uuid.UUID) (accessmodel.OrganizationRole, error) {
	role, err := r.q.GetOrganizationRole(ctx, sqlc.GetOrganizationRoleParams{OrganizationID: orgID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return role, dbError("get organization role", err)
}

func (r *InvitationsRepo) ContestRole(ctx context.Context, contestID, userID uuid.UUID) (accessmodel.ContestRole, error) {
	role, err := r.q.GetContestRole(ctx, sqlc.GetContestRoleParams{ContestID: contestID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return role, dbError("get contest role", err)
}

func (r *InvitationsRepo) JoinOrganization(ctx context.Context, orgID, userID uuid.UUID, role accessmodel.OrganizationRole) error {
	return dbError("join organization", r.q.JoinOrganization(ctx, sqlc.JoinOrganizationParams{OrganizationID: orgID, UserID: userID, Role: role}))
}

func (r *InvitationsRepo) JoinTeam(ctx context.Context, teamID, userID uuid.UUID) error {
	return dbError("join team", r.q.JoinTeam(ctx, sqlc.JoinTeamParams{TeamID: teamID, UserID: userID}))
}

func (r *InvitationsRepo) JoinContest(ctx context.Context, contestID, userID uuid.UUID) error {
	return dbError("join contest", r.q.JoinContest(ctx, sqlc.JoinContestParams{ContestID: contestID, UserID: userID}))
}
