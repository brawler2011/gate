package contests

import (
	"context"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

// StartVirtualRun begins the caller's replay of a finished contest with the contest's duration.
func (uc *ContestsUseCase) StartVirtualRun(ctx context.Context, contestID uuid.UUID) (contestsmodel.VirtualRun, error) {
	var run contestsmodel.VirtualRun
	err := access.WithinContest(ctx, uc.transactions, contestID, accessmodel.ContestStartVirtual, func(tx CommandTxStore, a access.ContestAccess) error {
		c := a.Facts.Contest
		run = contestsmodel.VirtualRun{UserID: a.Actor.Id, StartedAt: a.Facts.Now, EndsAt: a.Facts.Now.Add(c.EndTime.Sub(*c.StartTime))}
		return tx.CreateVirtualRun(ctx, contestID, run)
	})
	return run, err
}

// FinishVirtualRun ends the caller's run early; later submissions follow the upsolving rules.
func (uc *ContestsUseCase) FinishVirtualRun(ctx context.Context, contestID uuid.UUID) error {
	return access.WithinContest(ctx, uc.transactions, contestID, accessmodel.ContestView, func(tx CommandTxStore, a access.ContestAccess) error {
		if !a.Replaying() {
			return pkg.Wrap(pkg.ErrConflict, nil, "no virtual run is in progress")
		}
		return tx.FinishVirtualRun(ctx, contestID, a.Actor.Id, a.Facts.Now)
	})
}

// virtualScoreboard ranks virtual rows, each against the contest shifted to its run, with the official ones.
// During the caller's run every row is cut at the caller's elapsed time and frozen by that relative clock.
func (uc *ContestsUseCase) virtualScoreboard(ctx context.Context, a access.ContestAccess, unfrozen, others bool) (*contestsmodel.ScoreboardResponse, error) {
	contest, now := a.Facts.Contest, a.Facts.Now
	if contest.StartTime == nil || contest.EndTime == nil {
		return uc.ProjectScoreboard(ctx, contest.ID, unfrozen)
	}
	var elapsed *time.Duration
	if a.Replaying() {
		e := now.Sub(a.Facts.Run.StartedAt)
		elapsed = &e
	}
	// at is the moment a row starting at start is shown at: now, or the same elapsed time into it.
	at := func(start time.Time) time.Time {
		if elapsed == nil {
			return now
		}
		return start.Add(*elapsed)
	}
	problems, err := uc.contestRepo.GetContestProblems(ctx, contest.ID)
	if err != nil {
		return nil, err
	}
	participants, err := uc.contestRepo.ListContestParticipants(ctx, contest.ID)
	if err != nil {
		return nil, err
	}
	runs, err := uc.contestRepo.ListVirtualRuns(ctx, contest.ID)
	if err != nil {
		return nil, err
	}
	replays, err := uc.contestRepo.GetVirtualScoreboardSubmissions(ctx, contest.ID)
	if err != nil {
		return nil, err
	}

	shown := at(*contest.StartTime)
	rules := newScoreboardRules(contest, !unfrozen && contest.IsFrozenAt(shown))
	if elapsed != nil {
		rules.cutoff = &shown
	}
	official, err := uc.contestRepo.GetContestScoreboardSubmissions(ctx, contest.ID)
	if err != nil {
		return nil, err
	}
	items := make([]contestsmodel.ScoreboardItem, 0, len(participants)+len(runs))
	results := rulesResults(rules, problems, participants, official)
	for id, username := range participants {
		items = append(items, scoreboardItem(id, username, nonNil(results[id])))
	}
	response := uc.scoreboardResponse(contest, problems, rules.frozen, contest.GetFreezeTime())
	offsets := map[uuid.UUID]time.Duration{}
	for _, run := range runs {
		own := run.UserID == a.Actor.Id
		if !own && !others {
			continue
		}
		shifted := contest.Shift(run)
		offsets[run.UserID] = run.StartedAt.Sub(*contest.StartTime)
		cutoff := run.Over()
		if t := at(run.StartedAt); t.Before(cutoff) {
			cutoff = t
		}
		runRules := newScoreboardRules(shifted, !unfrozen && shifted.IsFrozenAt(at(run.StartedAt)))
		runRules.cutoff = &cutoff
		user := map[uuid.UUID]string{run.UserID: run.Username}
		item := scoreboardItem(run.UserID, run.Username, nonNil(rulesResults(runRules, problems, user, ownSubmissions(replays, run.UserID))[run.UserID]))
		item.Virtual = true
		items = append(items, item)
		if own && elapsed != nil {
			response.IsFrozen, response.FreezeTime = runRules.frozen, shifted.GetFreezeTime()
		}
	}
	// Ties on the last acceptance compare elapsed time, so virtual rows move onto the official timeline.
	sortRanked(items, func(item contestsmodel.ScoreboardItem) *time.Time {
		if !item.Virtual || item.LastAcceptedAt == nil {
			return item.LastAcceptedAt
		}
		t := item.LastAcceptedAt.Add(-offsets[item.UserID])
		return &t
	})
	response.Items = items
	return response, nil
}

func (uc *ContestsUseCase) scoreboardResponse(contest contestsmodel.Contest, problems []contestsmodel.ContestProblem, frozen bool, freezeTime *time.Time) *contestsmodel.ScoreboardResponse {
	headers := make([]contestsmodel.ScoreboardProblemHeader, len(problems))
	for i, p := range problems {
		headers[i] = contestsmodel.ScoreboardProblemHeader{ProblemID: p.ProblemID, Title: p.Title, ShortName: p.ShortName, Ordinal: int32(p.Ordinal)} // #nosec G115
	}
	return &contestsmodel.ScoreboardResponse{ContestID: contest.ID, ContestLogin: contest.Login, OrganizationLogin: contest.OrganizationLogin,
		PenaltyPerAttempt: contestsmodel.PenaltyPerAttempt, IsFrozen: frozen, FreezeTime: freezeTime, Problems: headers}
}

func ownSubmissions(subs []contestsmodel.ScoreboardSubmission, user uuid.UUID) []contestsmodel.ScoreboardSubmission {
	var own []contestsmodel.ScoreboardSubmission
	for _, sub := range subs {
		if sub.UserID == user {
			own = append(own, sub)
		}
	}
	return own
}

func nonNil(results []contestsmodel.ContestProblemResult) []contestsmodel.ContestProblemResult {
	if results == nil {
		return []contestsmodel.ContestProblemResult{}
	}
	return results
}
