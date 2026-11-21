// Package accesstest serves fixed authorization facts so unit tests run the real policy.
package accesstest

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/access"
	m "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

// Loader returns the caller from the context unless Users overrides it; Roles grants per resource and actor.
type Loader struct {
	Users         map[uuid.UUID]m.User
	Organizations map[uuid.UUID]access.OrganizationFacts
	Teams         map[uuid.UUID]access.TeamFacts
	Problems      map[uuid.UUID]access.ProblemFacts
	Contests      map[uuid.UUID]access.ContestFacts
	Roles         map[uuid.UUID]map[uuid.UUID]string
	Err           error
}

var _ access.Loader = Loader{}

// Grant gives actor role on resource.
func (l *Loader) Grant(resource, actor uuid.UUID, role string) {
	if l.Roles == nil {
		l.Roles = map[uuid.UUID]map[uuid.UUID]string{}
	}
	if l.Roles[resource] == nil {
		l.Roles[resource] = map[uuid.UUID]string{}
	}
	l.Roles[resource][actor] = role
}

func (l Loader) Actor(ctx context.Context, id uuid.UUID) (m.User, error) {
	if l.Err != nil {
		return m.User{}, l.Err
	}
	if u, ok := l.Users[id]; ok {
		return u, nil
	}
	if u := m.UserFromContext(ctx); u.Id == id {
		return u, nil
	}
	return m.User{}, pkg.ErrNotFound
}

func facts[F any](l Loader, all map[uuid.UUID]F, actor uuid.UUID, ids []uuid.UUID, role func(*F, string)) (map[uuid.UUID]F, error) {
	if l.Err != nil {
		return nil, l.Err
	}
	result := map[uuid.UUID]F{}
	for _, id := range ids {
		f, ok := all[id]
		if !ok {
			continue
		}
		role(&f, l.Roles[id][actor])
		result[id] = f
	}
	return result, nil
}

func (l Loader) OrganizationFacts(_ context.Context, actor uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]access.OrganizationFacts, error) {
	return facts(l, l.Organizations, actor, ids, func(f *access.OrganizationFacts, r string) { f.Role = m.OrganizationRole(r) })
}

func (l Loader) TeamFacts(_ context.Context, actor uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]access.TeamFacts, error) {
	return facts(l, l.Teams, actor, ids, func(f *access.TeamFacts, r string) { f.Role, f.Member = m.TeamRole(r), r != "" })
}

func (l Loader) ProblemFacts(_ context.Context, actor uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]access.ProblemFacts, error) {
	return facts(l, l.Problems, actor, ids, func(f *access.ProblemFacts, r string) { f.Role = m.ProblemRole(r) })
}

func (l Loader) ContestFacts(_ context.Context, actor uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]access.ContestFacts, error) {
	return facts(l, l.Contests, actor, ids, func(f *access.ContestFacts, r string) { f.Role = m.ContestRole(r) })
}
