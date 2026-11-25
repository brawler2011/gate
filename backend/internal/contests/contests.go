package contests

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	eventdata "github.com/brawler2011/gate/backend/internal/events"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"github.com/brawler2011/gate/backend/pkg/points"
	"github.com/google/uuid"
)

type ContestsUseCase struct {
	contestRepo     ContestsRepo
	orgsRepo        OrganizationsRepo
	transactions    CommandTransactions
	access          access.Loader
	notificationsUC NotificationsUC
}

func NewContestsUseCase(contestRepo ContestsRepo, orgsRepo OrganizationsRepo, transactions CommandTransactions, loader access.Loader) *ContestsUseCase {
	return &ContestsUseCase{
		contestRepo:  contestRepo,
		orgsRepo:     orgsRepo,
		transactions: transactions,
		access:       loader,
	}
}

// GetContest reads a contest without authorization; callers must not expose it.
func (uc *ContestsUseCase) GetContest(ctx context.Context, id uuid.UUID) (contestsmodel.Contest, error) {
	return uc.contestRepo.GetContest(ctx, id)
}

func (uc *ContestsUseCase) SetNotificationsUC(n NotificationsUC) {
	uc.notificationsUC = n
}

func (uc *ContestsUseCase) CreateContest(
	ctx context.Context,
	c *contestsmodel.CreateContestInput,
) (uuid.UUID, error) {
	params := &contestsmodel.CreateContestParams{
		ID:             uuid.New(),
		OrganizationID: c.OrganizationID,
		Visibility:     c.Visibility,
		Title:          c.Title,
		Login:          c.Login,
		Description:    c.Description,
		StartTime:      c.StartTime,
		EndTime:        c.EndTime,
	}

	err := access.WithinOrganization(ctx, uc.transactions, c.OrganizationID, accessmodel.OrganizationCreate, func(tx CommandTxStore, org access.OrganizationAccess) error {
		if err := tx.CreateContest(ctx, params); err != nil {
			return fmt.Errorf("create contest: %w", err)
		}
		return tx.CreateContestMember(ctx, &contestsmodel.CreateContestMemberParams{ContestId: params.ID, UserId: org.Actor.Id, Role: accessmodel.ContestOwner})
	})
	if err != nil {
		return uuid.Nil, err
	}

	return params.ID, nil
}

func (uc *ContestsUseCase) UpdateContest(ctx context.Context, c contestsmodel.ContestUpdateInput) error {
	return access.WithinContest(ctx, uc.transactions, c.ID, accessmodel.ContestManage, func(tx CommandTxStore, _ access.ContestAccess) error {
		if err := tx.UpdateContest(ctx, c); err != nil {
			return err
		}
		return tx.QueueBooklets(ctx, c.ID)
	})
}

func (uc *ContestsUseCase) DeleteContest(ctx context.Context, id uuid.UUID) error {
	return access.WithinContest(ctx, uc.transactions, id, accessmodel.ContestDelete, func(tx CommandTxStore, _ access.ContestAccess) error {
		return tx.DeleteContest(ctx, id)
	})
}

func (uc *ContestsUseCase) CreateContestProblem(ctx context.Context, c contestsmodel.ContestProblemCreation) error {
	return access.WithinContest(ctx, uc.transactions, c.ContestId, accessmodel.ContestManage, func(tx CommandTxStore, _ access.ContestAccess) error {
		var err error
		if c.PackageId, err = usableProblemPackage(ctx, tx, c.ProblemId, c.PackageId, true); err != nil {
			return err
		}
		if err := tx.LockContestProblemOrder(ctx, c.ContestId); err != nil {
			return err
		}
		if err := tx.CreateContestProblem(ctx, c); err != nil {
			return err
		}
		return tx.QueueBooklets(ctx, c.ContestId)
	})
}

// UpdateContestProblemPoints sets what solving a pass-fail problem scores under points ranking.
func (uc *ContestsUseCase) UpdateContestProblemPoints(ctx context.Context, contestID, problemID uuid.UUID, value points.Value) error {
	return access.WithinContest(ctx, uc.transactions, contestID, accessmodel.ContestManage, func(tx CommandTxStore, _ access.ContestAccess) error {
		problem, err := tx.GetContestProblem(ctx, contestsmodel.ContestProblemGet{ContestId: contestID, ProblemId: problemID})
		if err != nil {
			return err
		}
		if problem.Grading == spec.GradingPoints {
			return pkg.Wrap(pkg.ErrInvalid, nil, "a points problem scores its package max score")
		}
		return tx.UpdateContestProblemPoints(ctx, contestID, problemID, value)
	})
}

// Relinking still requires using the source problem, even during a running contest.
func (uc *ContestsUseCase) UpdateContestProblemPackage(ctx context.Context, contestId, problemId, packageId uuid.UUID) error {
	return access.WithinContest(ctx, uc.transactions, contestId, accessmodel.ContestManage, func(tx CommandTxStore, _ access.ContestAccess) error {
		var err error
		if packageId, err = usableProblemPackage(ctx, tx, problemId, packageId, false); err != nil {
			return err
		}
		if err := tx.UpdateContestProblemPackage(ctx, contestId, problemId, packageId); err != nil {
			return err
		}
		return tx.QueueBooklets(ctx, contestId)
	})
}

func (uc *ContestsUseCase) DeleteContestProblem(ctx context.Context, c contestsmodel.ContestProblemDeletion) error {
	return access.WithinContest(ctx, uc.transactions, c.ContestId, accessmodel.ContestManage, func(tx CommandTxStore, a access.ContestAccess) error {
		if err := tx.LockContestProblemOrder(ctx, c.ContestId); err != nil {
			return err
		}
		if err := tx.DeleteContestProblem(ctx, c); err != nil {
			return err
		}
		return tx.QueueBooklets(ctx, c.ContestId)
	})
}

func (uc *ContestsUseCase) ReorderContestProblems(ctx context.Context, contestId uuid.UUID, problems []contestsmodel.ContestProblemReorderItem) error {
	return access.WithinContest(ctx, uc.transactions, contestId, accessmodel.ContestManage, func(tx CommandTxStore, a access.ContestAccess) error {
		if err := tx.LockContestProblemOrder(ctx, contestId); err != nil {
			return err
		}
		if err := tx.ReorderContestProblems(ctx, contestId, problems); err != nil {
			return err
		}
		return tx.QueueBooklets(ctx, contestId)
	})
}

func (uc *ContestsUseCase) DeleteParticipant(ctx context.Context, c contestsmodel.ParticipantDeletion) error {
	return uc.changeMember(ctx, c.ContestId, c.UserId, "", func(tx CommandTxStore) error { return tx.DeleteContestMember(ctx, c.UserId, c.ContestId) })
}

func (uc *ContestsUseCase) UpdateContestMember(ctx context.Context, contestID, userID uuid.UUID, role accessmodel.ContestRole) error {
	return uc.changeMember(ctx, contestID, userID, role, func(tx CommandTxStore) error { return tx.UpdateContestMember(ctx, contestID, userID, role) })
}

// Members may lower or drop their own role without managing the contest.
func (uc *ContestsUseCase) changeMember(ctx context.Context, contestID, userID uuid.UUID, role accessmodel.ContestRole, write func(CommandTxStore) error) error {
	return access.WithinContest(ctx, uc.transactions, contestID, accessmodel.ContestView, func(tx CommandTxStore, a access.ContestAccess) error {
		member, err := tx.GetContestMember(ctx, &contestsmodel.ContestPermissionGet{ContestId: contestID, UserId: userID})
		if err != nil {
			return err
		}
		if err := a.ChangeMember(userID, member.ContestRole, role); err != nil {
			return err
		}
		return write(tx)
	})
}

func (uc *ContestsUseCase) ProjectScoreboard(ctx context.Context, contestID uuid.UUID, unfrozen bool) (*contestsmodel.ScoreboardResponse, error) {
	contest, err := uc.contestRepo.GetContest(ctx, contestID)
	if err != nil {
		return nil, err
	}
	problems, err := uc.contestRepo.GetContestProblems(ctx, contestID)
	if err != nil {
		return nil, err
	}
	users, err := uc.contestRepo.ListContestParticipants(ctx, contestID)
	if err != nil {
		return nil, err
	}
	subs, err := uc.contestRepo.GetContestScoreboardSubmissions(ctx, contestID)
	if err != nil {
		return nil, err
	}
	isFrozen := contest.IsFrozenAt(accessmodel.ReadTime(ctx))
	results := scoreboardResults(contest, problems, users, isFrozen && !unfrozen, subs)
	items := make([]contestsmodel.ScoreboardItem, 0, len(users))
	for id, username := range users {
		items = append(items, scoreboardItem(contest.Settings.Ranking, id, username, nonNil(results[id])))
	}
	rankItems(items, contest.Settings, func(_ contestsmodel.ScoreboardItem, t *time.Time) *time.Time { return t })
	response := uc.scoreboardResponse(contest, problems, isFrozen, contest.GetFreezeTime())
	response.Items = items
	return response, nil
}

// rankItems sorts items and numbers them, rows equal on every ranking key sharing a place; at moves a row's time onto the official timeline.
func rankItems(items []contestsmodel.ScoreboardItem, s contestsmodel.ContestSettings, at func(contestsmodel.ScoreboardItem, *time.Time) *time.Time) {
	compare := func(a, b contestsmodel.ScoreboardItem) int {
		if s.Ranking == contestsmodel.RankingPoints {
			if c := cmp.Compare(*b.Score, *a.Score); c != 0 || s.TieBreak != contestsmodel.TieBreakTime {
				return c
			}
			return compareTimes(at(a, a.ScoredAt), at(b, b.ScoredAt))
		}
		if c := cmp.Compare(b.ProblemsSolved, a.ProblemsSolved); c != 0 {
			return c
		}
		if c := cmp.Compare(a.TotalPenalty, b.TotalPenalty); c != 0 {
			return c
		}
		return compareTimes(at(a, a.LastAcceptedAt), at(b, b.LastAcceptedAt))
	}
	slices.SortStableFunc(items, func(a, b contestsmodel.ScoreboardItem) int {
		if c := compare(a, b); c != 0 {
			return c
		}
		return strings.Compare(a.Username, b.Username)
	})
	for i := range items {
		items[i].Rank = int32(i + 1) // #nosec G115
		if i > 0 && compare(items[i-1], items[i]) == 0 {
			items[i].Rank = items[i-1].Rank
		}
	}
}

// compareTimes orders earlier times first and missing ones last.
func compareTimes(a, b *time.Time) int {
	switch {
	case a != nil && b != nil:
		return a.Compare(*b)
	case a != nil:
		return -1
	case b != nil:
		return 1
	}
	return 0
}

func (uc *ContestsUseCase) CreateContestTeam(
	ctx context.Context,
	contestID, teamID uuid.UUID,
	role accessmodel.ContestRole,
) error {
	return access.WithinContest(ctx, uc.transactions, contestID, accessmodel.ContestManage, func(tx CommandTxStore, a access.ContestAccess) error {
		if err := a.ChangeTeam("", role); err != nil {
			return err
		}
		return tx.CreateContestTeam(ctx, contestID, teamID, role)
	})
}

func (uc *ContestsUseCase) UpdateContestTeamRole(
	ctx context.Context,
	contestID, teamID uuid.UUID,
	role accessmodel.ContestRole,
) error {
	return access.WithinContest(ctx, uc.transactions, contestID, accessmodel.ContestManage, func(tx CommandTxStore, a access.ContestAccess) error {
		link, err := tx.GetContestTeam(ctx, contestID, teamID)
		if err != nil {
			return err
		}
		if err := a.ChangeTeam(link.Role, role); err != nil {
			return err
		}
		return tx.UpdateContestTeamRole(ctx, contestID, teamID, role)
	})
}

func (uc *ContestsUseCase) DeleteContestTeam(
	ctx context.Context,
	contestID, teamID uuid.UUID,
) error {
	return access.WithinContest(ctx, uc.transactions, contestID, accessmodel.ContestManage, func(tx CommandTxStore, a access.ContestAccess) error {
		link, err := tx.GetContestTeam(ctx, contestID, teamID)
		if err != nil {
			return err
		}
		if err := a.ChangeTeam(link.Role, ""); err != nil {
			return err
		}
		return tx.DeleteContestTeam(ctx, contestID, teamID)
	})
}

func (uc *ContestsUseCase) BlockProblemForUser(ctx context.Context, contestID, userID, problemID uuid.UUID, reason *string) error {
	operatorID := accessmodel.UserFromContext(ctx).Id
	return access.WithinContest(ctx, uc.transactions, contestID, accessmodel.ContestManage, func(tx CommandTxStore, a access.ContestAccess) error {
		if err := tx.LockSubmissionTuple(ctx, contestID, userID, problemID); err != nil {
			return err
		}
		if err := tx.CreateContestUserProblemBlock(ctx, &contestsmodel.CreateContestUserProblemBlockParams{ContestID: contestID, UserID: userID, ProblemID: problemID, Reason: reason, CreatedBy: &operatorID}); err != nil {
			return err
		}
		changed, err := tx.BlockUserProblemSubmissions(ctx, contestID, userID, problemID, reason)
		if err != nil {
			return err
		}
		for _, change := range changed {
			if err := tx.Notify(ctx, eventdata.Notice{Type: submissionsmodel.SubmissionEventCompleted, ID: change.ID}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (uc *ContestsUseCase) UnblockProblemForUser(ctx context.Context, contestID, userID, problemID uuid.UUID, rejudgeSubmissions bool) error {
	return access.WithinContest(ctx, uc.transactions, contestID, accessmodel.ContestManage, func(tx CommandTxStore, a access.ContestAccess) error {
		if err := tx.LockSubmissionTuple(ctx, contestID, userID, problemID); err != nil {
			return err
		}
		if err := tx.DeleteContestUserProblemBlock(ctx, contestID, userID, problemID); err != nil {
			return err
		}
		if rejudgeSubmissions {
			changed, err := tx.ResetSubmissionsState(ctx, submissionsmodel.RejudgeFilter{ContestID: contestID, UserID: &userID, ProblemID: &problemID})
			if err != nil {
				return err
			}
			for _, change := range changed {
				if _, err := tx.ScheduleJob(ctx, change.ID); err != nil {
					return err
				}
				if err := tx.Notify(ctx, eventdata.Notice{Type: submissionsmodel.SubmissionEventCreated, ID: change.ID}); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (uc *ContestsUseCase) GetProblemBlockStatusForUser(
	ctx context.Context,
	contestID, userID, problemID uuid.UUID,
) (*contestsmodel.ContestUserProblemBlock, error) {
	a, err := access.Contest(ctx, uc.access, contestID, accessmodel.ContestView)
	if err != nil {
		return nil, err
	}
	if a.Actor.Id != userID && !a.Can(accessmodel.ContestManage) {
		return nil, pkg.ErrForbidden
	}
	block, err := uc.contestRepo.GetContestUserProblemBlock(ctx, contestID, userID, problemID)
	if err != nil {
		if errors.Is(err, pkg.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return block, nil
}

func (uc *ContestsUseCase) CreateJoinRequest(
	ctx context.Context,
	contestID uuid.UUID,
	message *string,
) (*contestsmodel.ContestJoinRequest, bool, error) {
	user, err := accessmodel.RequireUser(ctx)
	if err != nil {
		return nil, false, err
	}
	userID := user.Id
	var req *contestsmodel.ContestJoinRequest
	registered := false
	var pendingNotifications []notificationsmodel.Notification
	err = access.WithinContest(ctx, uc.transactions, contestID, accessmodel.ContestView, func(tx CommandTxStore, current access.ContestAccess) error {
		contest := current.Facts.Contest
		_, err := tx.GetContestMember(ctx, &contestsmodel.ContestPermissionGet{ContestId: contest.ID, UserId: userID})
		if err == nil {
			return pkg.Wrap(pkg.ErrAlreadyMember, nil, "already a member of the contest")
		}
		if !errors.Is(err, pkg.ErrNotFound) {
			return err
		}
		mode := contest.Settings.JoinPolicy
		if mode == accessmodel.JoinInviteOnly {
			return pkg.Wrap(pkg.ErrInviteOnly, nil, "the contest accepts participants by invitation only")
		}
		if mode == accessmodel.JoinOpen {
			if _, err := tx.GetMember(ctx, contest.OrganizationID, userID); err != nil {
				return pkg.OnNotFound(err, pkg.ErrNotOrgMember, "user must be a member of the organization")
			}
			if err := tx.CreateContestMember(ctx, &contestsmodel.CreateContestMemberParams{ContestId: contest.ID, UserId: userID, Role: accessmodel.ContestParticipant}); err != nil {
				return err
			}
			registered = true
			return nil
		}
		pending, err := tx.GetPendingContestJoinRequest(ctx, contest.ID, userID)
		if err != nil {
			return err
		}
		if pending != nil {
			return pkg.Wrap(pkg.ErrRequestPending, nil, "a join request is already pending")
		}
		req, err = tx.CreateContestJoinRequest(ctx, &contestsmodel.CreateContestJoinRequestInput{ContestID: contest.ID, UserID: userID, Message: message})
		if err != nil {
			return err
		}
		owners, err := tx.ListContestOwners(ctx, contest.ID)
		if err != nil {
			return err
		}
		link := fmt.Sprintf("/%s/%s", contest.OrganizationLogin, contest.Login)
		for _, owner := range owners {
			notification, err := tx.CreateNotification(ctx, &notificationsmodel.CreateNotificationInput{
				UserID: owner,
				Type:   notificationsmodel.NotificationTypeContestJoinRequest,
				Title:  fmt.Sprintf("Новая заявка на участие в контесте %s", contest.Title),
				Body:   fmt.Sprintf("Пользователь @%s запросил доступ к контесту %s", user.Username, contest.Title),
				Link:   &link,
				Data: map[string]interface{}{
					"contest_id":         contest.ID.String(),
					"contest_title":      contest.Title,
					"contest_login":      contest.Login,
					"organization_login": contest.OrganizationLogin,
					"applicant_id":       userID.String(),
					"applicant_username": user.Username,
					"request_id":         req.ID.String(),
				},
			})
			if err != nil {
				return err
			}
			pendingNotifications = append(pendingNotifications, *notification)
		}

		return nil
	})
	if err != nil {
		return nil, false, err
	}
	if registered {
		return nil, true, nil
	}

	for _, notification := range pendingNotifications {
		if uc.notificationsUC != nil {
			uc.notificationsUC.SendEmail(ctx, notification)
		}
	}

	return req, false, nil
}

func (uc *ContestsUseCase) ListJoinRequests(
	ctx context.Context,
	contestID uuid.UUID,
) ([]contestsmodel.ContestJoinRequest, error) {
	if _, err := access.Contest(ctx, uc.access, contestID, accessmodel.ContestManage); err != nil {
		return nil, err
	}

	status := string(organizationsmodel.RequestStatusPending)
	return uc.contestRepo.ListContestJoinRequests(ctx, contestID, &status)
}

func (uc *ContestsUseCase) CancelJoinRequest(
	ctx context.Context,
	contestID, requestID uuid.UUID,
) error {
	return access.WithinActor(ctx, uc.transactions, func(tx CommandTxStore, user accessmodel.User) error {
		req, err := tx.GetContestJoinRequestByID(ctx, requestID)
		if err != nil {
			return err
		}
		if req.ContestID != contestID || req.UserID != user.Id {
			return pkg.Wrap(pkg.ErrNotFound, nil, "join request not found")
		}
		if req.Status != organizationsmodel.RequestStatusPending {
			return pkg.Wrap(pkg.ErrRequestResolved, nil, "the request is already resolved")
		}
		return tx.UpdateContestJoinRequestStatus(ctx, req.ID, organizationsmodel.RequestStatusCanceled, nil)
	})
}

func (uc *ContestsUseCase) ResolveJoinRequest(ctx context.Context, contestID, requestID uuid.UUID, approve bool) error {
	if approve {
		return uc.approveJoinRequest(ctx, contestID, requestID)
	}
	return uc.rejectJoinRequest(ctx, contestID, requestID)
}

func (uc *ContestsUseCase) approveJoinRequest(
	ctx context.Context,
	contestID uuid.UUID,
	requestID uuid.UUID,
) error {

	reviewerID := accessmodel.UserFromContext(ctx).Id

	var pendingNotifications []notificationsmodel.Notification
	err := access.WithinContest(ctx, uc.transactions, contestID, accessmodel.ContestManage, func(tx CommandTxStore, a access.ContestAccess) error {
		contest := a.Facts.Contest
		req, err := tx.GetContestJoinRequestByID(ctx, requestID)
		if err != nil {
			return err
		}
		if req.ContestID != contest.ID {
			return pkg.Wrap(pkg.ErrNotFound, nil, "request does not belong to this contest")
		}
		if req.Status != organizationsmodel.RequestStatusPending {
			return pkg.Wrap(pkg.ErrRequestResolved, nil, "the request is already resolved")
		}
		if err := a.ChangeMember(req.UserID, "", accessmodel.ContestParticipant); err != nil {
			return err
		}

		if err := tx.UpdateContestJoinRequestStatus(ctx, req.ID, organizationsmodel.RequestStatusApproved, &reviewerID); err != nil {
			return fmt.Errorf("update join request status: %w", err)
		}

		err = tx.CreateContestMember(ctx, &contestsmodel.CreateContestMemberParams{
			ContestId: contest.ID,
			UserId:    req.UserID,
			Role:      accessmodel.ContestParticipant,
		})
		if err != nil {
			return fmt.Errorf("create contest member: %w", err)
		}

		link := fmt.Sprintf("/%s/%s", contest.OrganizationLogin, contest.Login)
		notification, err := tx.CreateNotification(ctx, &notificationsmodel.CreateNotificationInput{
			UserID: req.UserID,
			Type:   notificationsmodel.NotificationTypeContestJoinApproved,
			Title:  fmt.Sprintf("Заявка на контест %s одобрена", contest.Title),
			Body:   fmt.Sprintf("Ваша заявка на участие в контесте %s была одобрена!", contest.Title),
			Link:   &link,
			Data: map[string]interface{}{
				"contest_id":         contest.ID.String(),
				"contest_title":      contest.Title,
				"contest_login":      contest.Login,
				"organization_login": contest.OrganizationLogin,
				"request_id":         req.ID.String(),
			},
		})
		if err != nil {
			return err
		}
		pendingNotifications = append(pendingNotifications, *notification)

		return nil
	})
	if err != nil {
		return err
	}

	for _, notification := range pendingNotifications {
		if uc.notificationsUC != nil {
			uc.notificationsUC.SendEmail(ctx, notification)
		}
	}

	return nil
}

func (uc *ContestsUseCase) rejectJoinRequest(
	ctx context.Context,
	contestID uuid.UUID,
	requestID uuid.UUID,
) error {

	reviewerID := accessmodel.UserFromContext(ctx).Id

	var pendingNotifications []notificationsmodel.Notification
	if err := access.WithinContest(ctx, uc.transactions, contestID, accessmodel.ContestManage, func(tx CommandTxStore, a access.ContestAccess) error {
		contest := a.Facts.Contest
		req, err := tx.GetContestJoinRequestByID(ctx, requestID)
		if err != nil {
			return err
		}
		if req.ContestID != contest.ID {
			return pkg.Wrap(pkg.ErrNotFound, nil, "request does not belong to this contest")
		}
		if req.Status != organizationsmodel.RequestStatusPending {
			return pkg.Wrap(pkg.ErrRequestResolved, nil, "the request is already resolved")
		}

		if err := tx.UpdateContestJoinRequestStatus(ctx, req.ID, organizationsmodel.RequestStatusRejected, &reviewerID); err != nil {
			return err
		}

		link := fmt.Sprintf("/%s/%s", contest.OrganizationLogin, contest.Login)
		notification, err := tx.CreateNotification(ctx, &notificationsmodel.CreateNotificationInput{
			UserID: req.UserID,
			Type:   notificationsmodel.NotificationTypeContestJoinRejected,
			Title:  fmt.Sprintf("Заявка на контест %s отклонена", contest.Title),
			Body:   fmt.Sprintf("Ваша заявка на участие в контесте %s была отклонена.", contest.Title),
			Link:   &link,
			Data: map[string]interface{}{
				"contest_id":         contest.ID.String(),
				"contest_title":      contest.Title,
				"contest_login":      contest.Login,
				"organization_login": contest.OrganizationLogin,
				"request_id":         req.ID.String(),
			},
		})
		if err != nil {
			return err
		}
		pendingNotifications = append(pendingNotifications, *notification)

		return nil
	}); err != nil {
		return err
	}

	for _, notification := range pendingNotifications {
		if uc.notificationsUC != nil {
			uc.notificationsUC.SendEmail(ctx, notification)
		}
	}

	return nil
}

func (uc *ContestsUseCase) GetMyPendingJoinRequest(
	ctx context.Context,
	contestID uuid.UUID,
) (*contestsmodel.ContestJoinRequest, error) {
	user, err := accessmodel.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	return uc.contestRepo.GetPendingContestJoinRequest(ctx, contestID, user.Id)
}

func scoreboardItem(ranking contestsmodel.Ranking, userID uuid.UUID, username string, pResults []contestsmodel.ContestProblemResult) contestsmodel.ScoreboardItem {
	item := contestsmodel.ScoreboardItem{UserID: userID, Username: username, ProblemResults: pResults}
	if ranking == contestsmodel.RankingPoints {
		var total points.Value
		for _, r := range pResults {
			total += *r.Score
			if r.ScoredAt != nil && (item.ScoredAt == nil || r.ScoredAt.After(*item.ScoredAt)) {
				item.ScoredAt = r.ScoredAt
			}
		}
		item.Score = &total
	}
	for _, r := range pResults {
		if !r.Solved {
			continue
		}
		item.ProblemsSolved++
		if ranking != contestsmodel.RankingPoints {
			item.TotalPenalty += *r.TimeMinutes + r.Penalty
		}
		if r.FirstACTime != nil && (item.LastAcceptedAt == nil || r.FirstACTime.After(*item.LastAcceptedAt)) {
			item.LastAcceptedAt = r.FirstACTime
		}
	}
	return item
}

func (uc *ContestsUseCase) GetContestScoreboardUser(ctx context.Context, contestID, userID uuid.UUID, unfrozen bool) (*contestsmodel.ScoreboardResponse, error) {
	contest, err := uc.contestRepo.GetContest(ctx, contestID)
	if err != nil {
		return nil, err
	}
	member, err := uc.contestRepo.GetContestMember(ctx, &contestsmodel.ContestPermissionGet{ContestId: contestID, UserId: userID})
	if err != nil && !errors.Is(err, pkg.ErrNotFound) {
		return nil, err
	}
	problems, err := uc.contestRepo.GetContestProblems(ctx, contestID)
	if err != nil {
		return nil, err
	}
	response := uc.scoreboardResponse(contest, problems, contest.IsFrozenAt(accessmodel.ReadTime(ctx)), contest.GetFreezeTime())
	response.Items = []contestsmodel.ScoreboardItem{}
	if member.ContestRole != accessmodel.ContestParticipant {
		return response, nil
	}
	subs, err := uc.contestRepo.GetUserScoreboardSubmissions(ctx, contestID, userID)
	if err != nil {
		return nil, err
	}
	results := scoreboardResults(contest, problems, map[uuid.UUID]string{userID: member.Username}, response.IsFrozen && !unfrozen, subs)[userID]
	if results == nil {
		results = []contestsmodel.ContestProblemResult{}
	}
	response.Items = append(response.Items, scoreboardItem(contest.Settings.Ranking, userID, member.Username, results))
	return response, nil
}

func scoreboardResults(contest contestsmodel.Contest, problems []contestsmodel.ContestProblem, users map[uuid.UUID]string, frozen bool, subs []contestsmodel.ScoreboardSubmission) map[uuid.UUID][]contestsmodel.ContestProblemResult {
	return rulesResults(newScoreboardRules(contest, frozen), problems, users, subs)
}

func rulesResults(rules scoreboardRules, problems []contestsmodel.ContestProblem, users map[uuid.UUID]string, subs []contestsmodel.ScoreboardSubmission) map[uuid.UUID][]contestsmodel.ContestProblemResult {
	ordinals := make(map[uuid.UUID]int, len(problems))
	for i, problem := range problems {
		ordinals[problem.ProblemID] = i
	}
	results := make(map[uuid.UUID][]contestsmodel.ContestProblemResult, len(users))
	for start := 0; start < len(subs); {
		first := subs[start]
		end := start + 1
		for end < len(subs) && subs[end].UserID == first.UserID && subs[end].ProblemID == first.ProblemID {
			end++
		}
		_, participant := users[first.UserID]
		if i, ok := ordinals[first.ProblemID]; ok && participant {
			if result, ok := calculateScoreboardProblem(rules, first.UserID, problems[i], subs[start:end]); ok {
				if results[first.UserID] == nil {
					results[first.UserID] = make([]contestsmodel.ContestProblemResult, 0, len(problems))
				}
				results[first.UserID] = append(results[first.UserID], result)
			}
		}
		start = end
	}
	for _, row := range results {
		slices.SortFunc(row, func(a, b contestsmodel.ContestProblemResult) int {
			return ordinals[a.ProblemID] - ordinals[b.ProblemID]
		})
	}
	return results
}

type scoreboardRules struct {
	contest contestsmodel.Contest
	// cutoff hides submissions made at or after it, replaying the standings at an earlier moment.
	cutoff            *time.Time
	frozen            bool
	freezeTime        *time.Time
	manualFreeze      bool
	penaltyPerAttempt int32
}

func newScoreboardRules(contest contestsmodel.Contest, frozen bool) scoreboardRules {
	return scoreboardRules{contest: contest, frozen: frozen, freezeTime: contest.GetFreezeTime(), manualFreeze: contest.Settings.FreezeStatus == contestsmodel.FreezeStatusFrozen, penaltyPerAttempt: contestsmodel.PenaltyPerAttempt}
}

// earned is what a judged submission scores under points ranking.
func earned(problem contestsmodel.ContestProblem, sub contestsmodel.SubmissionForScoreboard) points.Value {
	switch {
	case sub.State == submissionsmodel.Accepted && problem.Grading != spec.GradingPoints:
		return problem.MaxScore
	case sub.State == submissionsmodel.Accepted || sub.State == submissionsmodel.Partial:
		return sub.Score
	}
	return 0
}

func calculateScoreboardProblem(rules scoreboardRules, userID uuid.UUID, problem contestsmodel.ContestProblem, subs []contestsmodel.ScoreboardSubmission) (contestsmodel.ContestProblemResult, bool) {
	contest, frozen, freezeTime := rules.contest, rules.frozen, rules.freezeTime
	scored := contest.Settings.Ranking == contestsmodel.RankingPoints

	var solved bool
	var failedAttempts, pendingAttempts, considered int32
	var firstACTime, scoredAt *time.Time
	var best points.Value
	minutes := func(t time.Time) *int32 {
		var m int32
		if contest.StartTime != nil {
			m = max(0, int32(t.Sub(*contest.StartTime).Minutes()))
		}
		return &m
	}

	for _, row := range subs {
		sub := row.SubmissionForScoreboard
		if contest.StartTime != nil && sub.CreatedAt.Before(*contest.StartTime) {
			continue
		}
		if contest.EndTime != nil && !sub.CreatedAt.Before(*contest.EndTime) {
			continue
		}
		if rules.cutoff != nil && !sub.CreatedAt.Before(*rules.cutoff) {
			continue
		}
		considered++

		if solved {
			continue
		}
		// ICPC freeze: verdicts of submissions made after the freeze stay hidden, earlier ones show whenever judged.
		if frozen && (freezeTime != nil && !sub.CreatedAt.Before(*freezeTime) || freezeTime == nil && rules.manualFreeze) {
			if !contest.Settings.FreezeHidesAttempts {
				pendingAttempts++
			}
			continue
		}
		if sub.State == submissionsmodel.Saved || sub.State == submissionsmodel.GotCE {
			continue
		}
		if score := earned(problem, sub); scored && score > best {
			best, scoredAt = score, &sub.CreatedAt
		}

		if sub.State == submissionsmodel.Accepted {
			solved = true
			firstACTime = &sub.CreatedAt
		} else if sub.State == submissionsmodel.GotTL || sub.State == submissionsmodel.GotML ||
			sub.State == submissionsmodel.GotRE || sub.State == submissionsmodel.GotPE || sub.State == submissionsmodel.GotWA || sub.State == submissionsmodel.Partial || sub.State == submissionsmodel.GotOL || sub.State == submissionsmodel.Disqualified {
			failedAttempts++
		}
	}

	// A replay must not reveal that a problem will be attempted later.
	seen := len(subs) > 0
	if rules.cutoff != nil {
		seen = considered > 0
	}
	if !solved && failedAttempts == 0 && pendingAttempts == 0 && (frozen || !seen) {
		return contestsmodel.ContestProblemResult{}, false
	}
	result := contestsmodel.ContestProblemResult{ContestID: contest.ID, UserID: userID, ProblemID: problem.ProblemID, Solved: solved,
		FailedAttempts: failedAttempts, PendingAttempts: pendingAttempts, FirstACTime: firstACTime}
	switch {
	case scored:
		result.Score, result.ScoredAt = &best, scoredAt
		if scoredAt != nil {
			result.TimeMinutes = minutes(*scoredAt)
		}
	case solved:
		result.TimeMinutes, result.Penalty = minutes(*firstACTime), failedAttempts*rules.penaltyPerAttempt
	}
	return result, true
}
