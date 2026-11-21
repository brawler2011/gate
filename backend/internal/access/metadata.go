package access

import (
	"context"
	"errors"

	m "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

// Missing resources are absent; failed fact reads must return an error.
type PolicyFacts interface {
	LoadProblemFacts(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]m.ProblemContext, error)
	LoadContestFacts(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]m.ContestContext, error)
	LoadTeamFacts(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]m.TeamContext, error)
}

type MetadataPermissions struct {
	*PermissionsUseCase
	facts PolicyFacts
}

func NewMetadataPermissions(legacy *PermissionsUseCase, facts PolicyFacts) *MetadataPermissions {
	return &MetadataPermissions{PermissionsUseCase: legacy, facts: facts}
}
func (uc *MetadataPermissions) input(ctx context.Context, id uuid.UUID) (m.PolicyInput, error) {
	in := m.PolicyInput{Actor: m.Guest, Now: m.ReadTime(ctx)}
	if id != uuid.Nil {
		user, err := uc.users.GetUserById(ctx, id)
		if err != nil {
			return in, err
		}
		in.Actor = user
	}
	return in, validateInput(in)
}
func metadataDecision(err error) (bool, error) {
	if errors.Is(err, pkg.ErrForbidden) {
		return false, nil
	}
	return err == nil, err
}
func (uc *MetadataPermissions) CanProblem(ctx context.Context, id uuid.UUID, action m.ProblemAction) (bool, error) {
	user := m.UserFromContext(ctx).Id
	if action != m.ActionViewProblem && action != m.ActionUseProblem {
		return uc.PermissionsUseCase.CanProblem(ctx, id, action)
	}
	in, err := uc.input(ctx, user)
	if err != nil {
		return false, err
	}
	facts, err := uc.facts.LoadProblemFacts(ctx, user, []uuid.UUID{id})
	if err != nil {
		return false, err
	}
	c, ok := facts[id]
	if !ok {
		return false, pkg.ErrNotFound
	}
	return metadataDecision(RequireProblem(in, c, action))
}
func (uc *MetadataPermissions) CanContest(ctx context.Context, id uuid.UUID, action m.ContestAction) (bool, error) {
	user := m.UserFromContext(ctx).Id
	in, err := uc.input(ctx, user)
	if err != nil {
		return false, err
	}
	facts, err := uc.facts.LoadContestFacts(ctx, user, []uuid.UUID{id})
	if err != nil {
		return false, err
	}
	c, ok := facts[id]
	if !ok {
		return false, pkg.ErrNotFound
	}
	if err := ApplyContestLifecycle(in, &c); err != nil {
		return metadataDecision(err)
	}
	return metadataDecision(RequireContest(in, c, action))
}
func (uc *MetadataPermissions) CanTeam(ctx context.Context, id uuid.UUID, action m.TeamAction) (bool, error) {
	user := m.UserFromContext(ctx).Id
	in, err := uc.input(ctx, user)
	if err != nil {
		return false, err
	}
	facts, err := uc.facts.LoadTeamFacts(ctx, user, []uuid.UUID{id})
	if err != nil {
		return false, err
	}
	c, ok := facts[id]
	if !ok {
		return false, pkg.ErrNotFound
	}
	return metadataDecision(RequireTeam(in, c, action))
}

func (uc *MetadataPermissions) AuthorizeContestAudience(ctx context.Context, id uuid.UUID, audience string, owner uuid.UUID) error {
	user := m.UserFromContext(ctx).Id
	in, err := uc.input(ctx, user)
	if err != nil {
		return err
	}
	facts, err := uc.facts.LoadContestFacts(ctx, user, []uuid.UUID{id})
	if err != nil {
		return err
	}
	c, ok := facts[id]
	if !ok {
		return pkg.ErrNotFound
	}
	return RequireContestAudience(in, c, audience, owner)
}
