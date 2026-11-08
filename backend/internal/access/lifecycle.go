package access

import (
	"context"
	"math"
	"time"

	m "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

// ApplyContestLifecycle computes every temporal gate from the same server clock.
func ApplyContestLifecycle(in m.PolicyInput, c *m.ContestContext) error {
	if err := validateInput(in); err != nil {
		return err
	}
	if raw, ok := c.Settings["freeze_status"]; ok && raw != nil && raw != "" {
		status, valid := raw.(string)
		if !valid || status != contestsmodel.FreezeStatusAuto && status != contestsmodel.FreezeStatusFrozen && status != contestsmodel.FreezeStatusUnfrozen {
			return pkg.NoPermission
		}
	}
	if raw, ok := c.Settings["freeze_duration_minutes"]; ok && raw != nil {
		var minutes float64
		switch v := raw.(type) {
		case float64:
			minutes = v
		case int:
			minutes = float64(v)
		case int32:
			minutes = float64(v)
		case int64:
			minutes = float64(v)
		default:
			return pkg.NoPermission
		}
		if math.IsNaN(minutes) || math.IsInf(minutes, 0) || minutes < 0 || minutes != math.Trunc(minutes) || minutes > float64(math.MaxInt64/int64(time.Minute)) {
			return pkg.NoPermission
		}
	}
	if c.StartTime != nil && c.EndTime != nil && !c.StartTime.Before(*c.EndTime) {
		return pkg.NoPermission
	}
	_, err := contestPolicyRank(in, *c)
	if err != nil {
		return err
	}
	participant := !in.Actor.IsGuest() && (c.OwnerID == in.Actor.Id || c.DirectRole != "" || hasContestParticipation(*c) || c.Visibility == m.VisibilityPublic && c.ParticipationMode == m.ParticipationOpen)
	started := c.StartTime == nil || !in.Now.Before(*c.StartTime)
	finished := c.EndTime != nil && !in.Now.Before(*c.EndTime)
	audience := func(key, defaultScope string) (bool, error) {
		scope := defaultScope
		if raw, ok := c.Settings[key]; ok && raw != nil && raw != "" {
			var valid bool
			scope, valid = raw.(string)
			if !valid {
				return false, pkg.NoPermission
			}
		}
		switch scope {
		case "public":
			return started, nil
		case "participant":
			return started && participant, nil
		case "moderator":
			return false, nil
		default:
			return false, pkg.NoPermission
		}
	}
	t := &m.ContestConditions{Submit: participant && started && !finished}
	hidden := false
	if raw, ok := c.Settings["hide_statements"]; ok && raw != nil {
		var valid bool
		hidden, valid = raw.(bool)
		if !valid {
			return pkg.NoPermission
		}
	}
	t.ViewStatements = started && !hidden
	if t.ViewScoreboard, err = audience("monitor_scope", "participant"); err != nil {
		return err
	}
	if t.ListSubmissions, err = audience("submissions_list_scope", "moderator"); err != nil {
		return err
	}
	if t.ViewOtherSubmission, err = audience("submissions_review_scope", "moderator"); err != nil {
		return err
	}
	if t.ViewSubmissionDetails, err = audience("submission_details_scope", "moderator"); err != nil {
		return err
	}
	c.Conditions = t
	return nil
}

// RequireAttachProblem is also the relink policy, including during Running.
// Destination ownership never substitutes for source authorization.
func RequireAttachProblem(in m.PolicyInput, contest m.ContestContext, problem m.ProblemContext, pack PackageContext) error {
	if err := RequireContest(in, contest, m.ActionManageContest); err != nil {
		return err
	}
	if err := RequireProblem(in, problem, m.ActionUseProblem); err != nil {
		return err
	}
	return RequireReadyPackage(problem.ID, pack)
}

type PackageContext struct {
	ID, ProblemID uuid.UUID
	Status        string
}

// GetContestPermissions retains the legacy transport shape until the DTO PR.
// Every field delegates to the same typed policy and lifecycle gates.
func (uc *MetadataPermissions) GetContestPermissions(ctx context.Context, id, user uuid.UUID) (*m.ContestPermissions, error) {
	in, err := uc.input(ctx, user)
	if err != nil {
		return nil, err
	}
	facts, err := uc.facts.LoadContestFacts(ctx, user, []uuid.UUID{id})
	if err != nil {
		return nil, err
	}
	c, ok := facts[id]
	if !ok {
		return nil, pkg.ErrNotFound
	}
	if err := ApplyContestLifecycle(in, &c); err != nil {
		return nil, err
	}
	// These booleans describe audience access; the concrete submission's owner and
	// contest are checked by the submission use case before applying them.
	c.Submission = m.SubmissionContext{ContestID: id, OwnerID: uuid.New()}
	allowed := func(a m.ContestAction) bool { return RequireContest(in, c, a) == nil }
	p := &m.ContestPermissions{
		GetContest: allowed(m.ActionGetContest), UpdateContest: allowed(m.ActionUpdateContest), ManageContest: allowed(m.ActionManageContest), AdminContest: allowed(m.ActionAdminContest), GetMonitor: allowed(m.ActionGetMonitor), ListUsersSubmissions: allowed(m.ActionListUsersSubmissions), ListOwnSubmissions: allowed(m.ActionListOwnSubmissions), GetOtherUserSubmission: allowed(m.ActionGetOtherUserSubmission), GetSubmissionDetails: allowed(m.ActionGetSubmissionDetails), CreateSubmission: allowed(m.ActionCreateSubmission),
	}
	c.Submission.OwnerID = user
	p.GetSubmissionDetails = RequireContest(in, c, m.ActionGetSubmissionDetails) == nil
	p.GetOwnSubmission = RequireContest(in, c, m.ActionGetOwnSubmission) == nil
	return p, nil
}

// ContestFromFacts provides the shared domain time representation.
func ContestFromFacts(c m.ContestContext) contestsmodel.Contest {
	return contestsmodel.Contest{ID: c.ID, StartTime: c.StartTime, EndTime: c.EndTime, Settings: c.Settings}
}

// RelationshipPermissions must use pool-backed readers.
type RelationshipPermissions interface {
	HasContestPermission(context.Context, uuid.UUID, uuid.UUID, m.ContestAction) (bool, error)
	HasProblemPermission(context.Context, uuid.UUID, uuid.UUID, m.ProblemAction) (bool, error)
}

func AuthorizeContestProblem(ctx context.Context, p RelationshipPermissions, contest, problem uuid.UUID, loadPackage func() (PackageContext, error)) (uuid.UUID, error) {
	actor := m.UserFromContext(ctx)
	if err := validateInput(m.PolicyInput{Actor: actor, Now: m.ReadTime(ctx)}); err != nil {
		return uuid.Nil, err
	}
	if actor.IsGuest() {
		return uuid.Nil, pkg.ErrUnauthenticated
	}
	allowed, err := p.HasContestPermission(ctx, contest, actor.Id, m.ActionManageContest)
	if err != nil {
		return uuid.Nil, err
	}
	if !allowed {
		return uuid.Nil, pkg.NoPermission
	}
	allowed, err = p.HasProblemPermission(ctx, problem, actor.Id, m.ActionUseProblem)
	if err != nil {
		return uuid.Nil, err
	}
	if !allowed {
		return uuid.Nil, pkg.NoPermission
	}
	pack, err := loadPackage()
	if err != nil {
		return uuid.Nil, err
	}
	if err := RequireReadyPackage(problem, pack); err != nil {
		return uuid.Nil, err
	}
	return pack.ID, nil
}

func RequireReadyPackage(problem uuid.UUID, pack PackageContext) error {
	if pack.ID == uuid.Nil || pack.ProblemID != problem || pack.Status != "ready" {
		return pkg.ErrBadInput
	}
	return nil
}
