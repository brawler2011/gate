//go:build integration

package integration

import (
	"context"
	"net/http"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
)

// A virtual runner sees the official field replayed at its own elapsed time; others see runs only when asking for them.
func (s *IntegrationTestSuite) TestVirtualContest() {
	admin := s.createUser("admin_virtual", accessmodel.UserRoleAdmin)
	participant := s.createUser("participant_virtual", accessmodel.UserRoleUser)
	runner := s.createUser("runner_virtual", accessmodel.UserRoleUser)
	watcher := s.createUser("watcher_virtual", accessmodel.UserRoleUser)
	org := s.createOrganization("virtual-org", "Virtual Organization", admin.Id)
	problem, err := s.client.CreateProblem(withTestUser(s.ctx, admin.Id), corev1.CreateProblemParams{IdempotencyKey: uuid.NewString(),
		Title: "Virtual Problem", OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: "builtin:a-plus-b"})
	s.Require().NoError(err)
	contestID := uuid.New()
	s.Require().NoError(s.createContest(contestsmodel.CreateContestParams{ID: contestID, OrganizationID: org.ID, Title: "Virtual",
		Login: "virtual", Visibility: contestsmodel.ContestVisibilityPublic}, admin.Id))
	now := time.Now().Truncate(time.Second)
	start, end := now.Add(-3*time.Hour), now.Add(-time.Hour)
	s.Require().NoError(s.contestsRepo.UpdateContest(s.ctx, contestsmodel.ContestUpdateInput{ID: contestID,
		StartTime: &start, StartTimeSet: true, EndTime: &end, EndTimeSet: true}))
	path := corev1.UpdateContestParams{OrgLogin: org.Login, ContestLogin: "virtual"}
	s.Require().NoError(s.client.UpdateContest(withTestUser(s.ctx, admin.Id), &corev1.UpdateContestRequestModel{
		AllowVirtual: corev1.NewOptBool(true), EnableUpsolving: corev1.NewOptBool(false), MonitorScope: corev1.NewOptContestScope(corev1.ContestScopePublic)}, path))
	s.Require().NoError(s.contestsRepo.CreateContestProblem(s.ctx, contestsmodel.ContestProblemCreation{
		ContestId: contestID, ProblemId: problem.ID, PackageId: s.createDummyProblemPackage(problem.ID, org.ID)}))
	s.Require().NoError(s.contestsRepo.CreateContestMember(s.ctx, &contestsmodel.CreateContestMemberParams{
		ContestId: contestID, UserId: participant.Id, Role: accessmodel.ContestParticipant}))
	// The participant solved the problem half an hour into the official contest.
	_, err = s.dbPool.Exec(s.ctx, `INSERT INTO submissions(contest_id,owner_id,problem_id,kind,source,language,state,created_at)
		VALUES ($1,$2,$3,'contest','print(1)','python3',200,$4)`, contestID, participant.Id, problem.ID, start.Add(30*time.Minute))
	s.Require().NoError(err)
	s.Require().NoError(contests.ApplySubmissionResult(s.ctx, s.contestsRepo, &submissionsmodel.Submission{ContestID: &contestID, CreatedBy: &participant.Id, ProblemID: &problem.ID}))

	as := func(user uuid.UUID) context.Context { return withTestUser(s.ctx, user) }
	run := corev1.StartVirtualRunParams{OrgLogin: org.Login, ContestLogin: "virtual"}
	_, err = s.client.StartVirtualRun(as(participant.Id), run)
	s.Equal(http.StatusForbidden, s.getStatusCode(err), "official competitors cannot replay")
	contest, err := s.client.GetContest(as(runner.Id), corev1.GetContestParams{OrgLogin: org.Login, ContestLogin: "virtual"})
	s.Require().NoError(err)
	s.True(contest.Contest.Permissions.StartVirtual)
	started, err := s.client.StartVirtualRun(as(runner.Id), run)
	s.Require().NoError(err)
	s.Equal(end.Sub(start), started.EndsAt.Sub(started.StartedAt))
	_, err = s.client.StartVirtualRun(as(runner.Id), run)
	s.Equal(http.StatusForbidden, s.getStatusCode(err), "one run per contest")

	submit := func() (uuid.UUID, error) {
		resp, err := s.client.CreateSubmission(as(runner.Id), &corev1.CreateSubmissionRequestModel{Submission: "print(1)"},
			corev1.CreateSubmissionParams{ProblemID: problem.ID, OrganizationLogin: org.Login, ContestLogin: "virtual", Language: "python3"})
		if err != nil {
			return uuid.Nil, err
		}
		return resp.ID, nil
	}
	id, err := submit()
	s.Require().NoError(err, "a run accepts submissions with upsolving off")
	sub, err := s.client.GetSubmission(as(runner.Id), corev1.GetSubmissionParams{SubmissionID: id})
	s.Require().NoError(err)
	s.Equal(corev1.SubmissionKindVirtual, sub.Submission.Kind)

	board := func(user uuid.UUID, virtual bool) map[uuid.UUID]corev1.ScoreboardItemModel {
		resp, err := s.client.GetContestScoreboard(as(user), corev1.GetContestScoreboardParams{OrgLogin: org.Login, ContestLogin: "virtual", Virtual: corev1.NewOptBool(virtual)})
		s.Require().NoError(err)
		rows := map[uuid.UUID]corev1.ScoreboardItemModel{}
		for _, item := range resp.Items {
			rows[item.UserID] = item
		}
		return rows
	}
	replay := board(runner.Id, false)
	s.Require().Len(replay, 2)
	s.Zero(replay[participant.Id].ProblemsSolved, "the official solve happens later in the replayed timeline")
	s.True(replay[runner.Id].Virtual.Or(false))
	s.Equal(int32(1), board(watcher.Id, false)[participant.Id].ProblemsSolved)
	s.Len(board(watcher.Id, false), 1)
	all := board(watcher.Id, true)
	s.Require().Len(all, 2)
	s.True(all[runner.Id].Virtual.Or(false))

	// A run keeps its own two-hour window when the contest is later shortened.
	begun := time.Now().Add(-100 * time.Minute)
	_, err = s.dbPool.Exec(s.ctx, `UPDATE contest_virtual_runs SET started_at = $3, ends_at = $4 WHERE contest_id = $1 AND user_id = $2`,
		contestID, runner.Id, begun, begun.Add(end.Sub(start)))
	s.Require().NoError(err)
	_, err = s.dbPool.Exec(s.ctx, `UPDATE submissions SET state = 106, created_at = $2 WHERE id = $1`, id, begun.Add(90*time.Minute))
	s.Require().NoError(err)
	shorter := start.Add(time.Hour)
	s.Require().NoError(s.contestsRepo.UpdateContest(s.ctx, contestsmodel.ContestUpdateInput{ID: contestID, EndTime: &shorter, EndTimeSet: true}))
	late := board(watcher.Id, true)[runner.Id]
	s.Require().Len(late.ProblemResults, 1)
	s.Equal(int32(1), late.ProblemResults[0].FailedAttempts)

	finish := corev1.FinishVirtualRunParams{OrgLogin: org.Login, ContestLogin: "virtual"}
	s.Require().NoError(s.client.FinishVirtualRun(as(runner.Id), finish))
	s.Equal(http.StatusConflict, s.getStatusCode(s.client.FinishVirtualRun(as(runner.Id), finish)))
	_, err = submit()
	s.Equal(http.StatusConflict, s.getStatusCode(err), "after the run upsolving rules apply")
	s.Equal(int32(1), board(runner.Id, false)[participant.Id].ProblemsSolved, "a finished run shows the final standings")

	// A frozen running contest must not reveal post-freeze verdicts through the virtual view either.
	later := time.Now().Add(time.Hour)
	s.Require().NoError(s.contestsRepo.UpdateContest(s.ctx, contestsmodel.ContestUpdateInput{ID: contestID, EndTime: &later, EndTimeSet: true}))
	s.Require().NoError(s.client.UpdateContest(withTestUser(s.ctx, admin.Id), &corev1.UpdateContestRequestModel{
		FreezeStatus: corev1.NewOptUpdateContestRequestModelFreezeStatus(corev1.UpdateContestRequestModelFreezeStatusFrozen)}, path))
	second := s.createUser("second_virtual", accessmodel.UserRoleUser)
	s.Require().NoError(s.contestsRepo.CreateContestMember(s.ctx, &contestsmodel.CreateContestMemberParams{
		ContestId: contestID, UserId: second.Id, Role: accessmodel.ContestParticipant}))
	_, err = s.dbPool.Exec(s.ctx, `INSERT INTO submissions(contest_id,owner_id,problem_id,kind,source,language,state,created_at)
		VALUES ($1,$2,$3,'contest','print(1)','python3',200,$4)`, contestID, second.Id, problem.ID, time.Now())
	s.Require().NoError(err)
	for _, virtual := range []bool{false, true} {
		s.Zero(board(watcher.Id, virtual)[second.Id].ProblemsSolved, "virtual=%v", virtual)
	}
}
