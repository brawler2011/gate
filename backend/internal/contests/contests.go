package contests

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	eventdata "github.com/brawler2011/gate/backend/internal/events"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type ContestsUseCase struct {
	contestRepo     ContestsRepo
	orgsRepo        OrganizationsRepo
	transactions    CommandTransactions
	notificationsUC NotificationsUC
	usersRepo       UsersRepo
}

func NewContestsUseCase(contestRepo ContestsRepo, orgsRepo OrganizationsRepo, transactions CommandTransactions) *ContestsUseCase {
	return &ContestsUseCase{contestRepo: contestRepo, orgsRepo: orgsRepo, transactions: transactions}
}

func (uc *ContestsUseCase) SetNotificationsUC(n NotificationsUC) {
	uc.notificationsUC = n
}

func (uc *ContestsUseCase) SetUsersRepo(u UsersRepo) {
	uc.usersRepo = u
}

func (uc *ContestsUseCase) CreateContest(
	ctx context.Context,
	c *contestsmodel.CreateContestInput,
) (uuid.UUID, error) {
	params := &contestsmodel.CreateContestParams{
		ID:             uuid.New(),
		OrganizationID: c.OrganizationID,
		OwnerID:        c.OwnerID,
		Visibility:     c.Visibility,
		Title:          c.Title,
		Login:          c.Login,
		Description:    c.Description,
		Settings:       c.Settings,
		StartTime:      c.StartTime,
		EndTime:        c.EndTime,
	}

	err := uc.transactions.Within(ctx, func(tx CommandTxStore) error {
		err := tx.CreateContest(ctx, params)
		if err != nil {
			return pkg.Wrap(err, nil, "can't create contest")
		}

		// If an owner ID was provided, add them as a contest member with owner role
		if c.OwnerID != nil {
			err = tx.CreateContestMember(ctx, &contestsmodel.CreateContestMemberParams{
				ContestId: params.ID,
				UserId:    *c.OwnerID,
				Role:      contestsmodel.ContestRoleOwner,
			})
			if err != nil {
				return pkg.Wrap(err, nil, "can't create contest member")
			}
		}

		return nil
	})
	if err != nil {
		return uuid.Nil, err
	}

	return params.ID, nil
}

func (uc *ContestsUseCase) GetContest(ctx context.Context, id uuid.UUID) (contestsmodel.Contest, error) {
	contest, err := uc.contestRepo.GetContest(ctx, id)
	if err != nil {
		return contestsmodel.Contest{}, err
	}
	return contest, nil
}

// FIXME: ебать, что с названием??
func (uc *ContestsUseCase) GetContestByOrgLoginAndContestLogin(ctx context.Context, orgLogin, contestLogin string) (contestsmodel.Contest, error) {
	contest, err := uc.contestRepo.GetContestByOrgLoginAndContestLogin(ctx, orgLogin, contestLogin)
	if err != nil {
		return contestsmodel.Contest{}, err
	}
	return contest, nil
}

func (uc *ContestsUseCase) UpdateContest(ctx context.Context, c contestsmodel.ContestUpdateInput) error {
	params := contestsmodel.ContestUpdateParams(c)

	return uc.contestRepo.UpdateContest(ctx, params)
}

func (uc *ContestsUseCase) DeleteContest(ctx context.Context, id uuid.UUID) error {
	return uc.contestRepo.DeleteContest(ctx, id)
}

func (uc *ContestsUseCase) CreateContestProblem(ctx context.Context, c contestsmodel.ContestProblemCreation) error {
	return uc.contestRepo.CreateContestProblem(ctx, c)
}

func (uc *ContestsUseCase) GetContestProblem(ctx context.Context, c contestsmodel.ContestProblemGet) (contestsmodel.ContestProblem, error) {
	return uc.contestRepo.GetContestProblem(ctx, c)
}

func (uc *ContestsUseCase) GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]contestsmodel.ContestProblem, error) {
	return uc.contestRepo.GetContestProblems(ctx, contestId)
}

func (uc *ContestsUseCase) UpdateContestProblemPackage(ctx context.Context, contestId, problemId, packageId uuid.UUID) error {
	return uc.contestRepo.UpdateContestProblemPackage(ctx, contestId, problemId, packageId)
}

func (uc *ContestsUseCase) DeleteContestProblem(ctx context.Context, c contestsmodel.ContestProblemDeletion) error {
	return uc.contestRepo.DeleteContestProblem(ctx, c)
}

func (uc *ContestsUseCase) ReorderContestProblems(ctx context.Context, contestId uuid.UUID, problems []contestsmodel.ContestProblemReorderItem) error {
	return uc.contestRepo.ReorderContestProblems(ctx, contestId, problems)
}

func (uc *ContestsUseCase) CreateParticipant(ctx context.Context, c contestsmodel.ParticipantCreation) error {
	if uc.orgsRepo != nil {
		contest, err := uc.contestRepo.GetContest(ctx, c.ContestId)
		if err != nil {
			return err
		}
		_, err = uc.orgsRepo.GetMember(ctx, contest.OrganizationID, c.UserId)
		if err != nil {
			return pkg.Wrap(pkg.ErrBadInput, nil, "user must be a member of the organization")
		}
	}

	return uc.contestRepo.CreateContestMember(ctx, &contestsmodel.CreateContestMemberParams{
		ContestId: c.ContestId,
		UserId:    c.UserId,
		Role:      contestsmodel.ContestRoleParticipant,
	})
}

func (uc *ContestsUseCase) DeleteParticipant(ctx context.Context, c contestsmodel.ParticipantDeletion) error {
	return uc.contestRepo.DeleteContestMember(ctx, c.UserId, c.ContestId)
}

func (uc *ContestsUseCase) GetContestMember(ctx context.Context, c *contestsmodel.ContestPermissionGet) (contestsmodel.ContestMember, error) {
	return uc.contestRepo.GetContestMember(ctx, c)
}

func (uc *ContestsUseCase) UpdateContestMember(ctx context.Context, contestId uuid.UUID, userId uuid.UUID, role string) error {
	if role != contestsmodel.ContestRoleOwner && role != contestsmodel.ContestRoleModerator && role != contestsmodel.ContestRoleParticipant {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid role value")
	}

	currentMember, err := uc.GetContestMember(ctx, &contestsmodel.ContestPermissionGet{
		ContestId: contestId,
		UserId:    userId,
	})
	if err != nil {
		return pkg.Wrap(err, nil, "can't get contest member")
	}

	if currentMember.Role == contestsmodel.ContestRoleOwner {
		return pkg.Wrap(pkg.ErrBadInput, nil, "cannot change role from owner")
	}

	if role == contestsmodel.ContestRoleOwner {
		return pkg.Wrap(pkg.ErrBadInput, nil, "cannot change role to owner")
	}

	err = uc.contestRepo.UpdateContestMember(ctx, contestId, userId, role)
	if err != nil {
		return pkg.Wrap(err, nil, "can't update contest member")
	}

	return nil
}

func (uc *ContestsUseCase) ProcessSubmissionResult(ctx context.Context, submission *submissionsmodel.Submission) error {
	return ApplySubmissionResult(ctx, uc.contestRepo, submission)
}

func (uc *ContestsUseCase) GetContestScoreboard(ctx context.Context, contestID, userID uuid.UUID, unfrozen bool) (*contestsmodel.ScoreboardResponse, error) {
	contest, err := uc.contestRepo.GetContest(ctx, contestID)
	if err != nil {
		return nil, err
	}

	isFrozen := contest.IsFrozenAt(time.Now())
	freezeTime := contest.GetFreezeTime()
	penaltyPerAttempt := contest.GetPenaltyPerAttempt()

	problems, err := uc.contestRepo.GetContestProblems(ctx, contestID)
	if err != nil {
		return nil, err
	}

	problemHeaders := make([]contestsmodel.ScoreboardProblemHeader, len(problems))
	for i, p := range problems {
		problemHeaders[i] = contestsmodel.ScoreboardProblemHeader{
			ProblemID: p.ProblemID,
			Title:     p.Title,
			ShortName: p.ShortName,
			Ordinal:   int32(p.Ordinal), // #nosec G115
		}
	}

	if !isFrozen || unfrozen {
		results, userMap, err := uc.contestRepo.GetContestScoreboardFromStandings(ctx, contestID)
		if err != nil {
			return nil, err
		}

		userResults := make(map[uuid.UUID][]contestsmodel.ContestProblemResult)
		for _, res := range results {
			res.Penalty = 0
			if res.Solved {
				res.Penalty = res.FailedAttempts * penaltyPerAttempt
			}
			res.PendingAttempts = 0
			userResults[res.UserID] = append(userResults[res.UserID], res)
		}

		items := make([]contestsmodel.ScoreboardItem, 0, len(userMap))
		for uID, username := range userMap {
			pResults := userResults[uID]
			if pResults == nil {
				pResults = []contestsmodel.ContestProblemResult{}
			}

			items = append(items, scoreboardItem(uID, username, pResults))
		}

		sortScoreboardItems(items)

		return &contestsmodel.ScoreboardResponse{
			ContestID:         contestID,
			ContestLogin:      contest.Login,
			OrganizationLogin: contest.OrganizationLogin,
			PenaltyPerAttempt: penaltyPerAttempt,
			IsFrozen:          isFrozen,
			FreezeTime:        freezeTime,
			Problems:          problemHeaders,
			Items:             items,
		}, nil
	}

	_, userMap, err := uc.contestRepo.GetContestScoreboardFromStandings(ctx, contestID)
	if err != nil {
		return nil, err
	}

	items := make([]contestsmodel.ScoreboardItem, 0, len(userMap))
	for uID, username := range userMap {
		pResults, err := uc.scoreboardUserResults(ctx, contest, problems, uID, true)
		if err != nil {
			return nil, err
		}

		items = append(items, scoreboardItem(uID, username, pResults))
	}

	sortScoreboardItems(items)

	return &contestsmodel.ScoreboardResponse{
		ContestID:         contestID,
		ContestLogin:      contest.Login,
		OrganizationLogin: contest.OrganizationLogin,
		PenaltyPerAttempt: penaltyPerAttempt,
		IsFrozen:          isFrozen,
		FreezeTime:        freezeTime,
		Problems:          problemHeaders,
		Items:             items,
	}, nil
}

func sortScoreboardItems(items []contestsmodel.ScoreboardItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].ProblemsSolved != items[j].ProblemsSolved {
			return items[i].ProblemsSolved > items[j].ProblemsSolved
		}
		if items[i].TotalPenalty != items[j].TotalPenalty {
			return items[i].TotalPenalty < items[j].TotalPenalty
		}
		switch {
		case items[i].LastAcceptedAt != nil && items[j].LastAcceptedAt != nil:
			if !items[i].LastAcceptedAt.Equal(*items[j].LastAcceptedAt) {
				return items[i].LastAcceptedAt.Before(*items[j].LastAcceptedAt)
			}
		case items[i].LastAcceptedAt != nil:
			return true
		case items[j].LastAcceptedAt != nil:
			return false
		}
		return items[i].Username < items[j].Username
	})
}

func (uc *ContestsUseCase) CreateContestTeam(
	ctx context.Context,
	contestID, teamID, requestUserID uuid.UUID,
	role contestsmodel.ContestRole,
) error {
	return uc.contestRepo.CreateContestTeam(ctx, contestID, teamID, role)
}

func (uc *ContestsUseCase) UpdateContestTeamRole(
	ctx context.Context,
	contestID, teamID, requestUserID uuid.UUID,
	role contestsmodel.ContestRole,
) error {
	return uc.contestRepo.UpdateContestTeamRole(ctx, contestID, teamID, role)
}

func (uc *ContestsUseCase) DeleteContestTeam(
	ctx context.Context,
	contestID, teamID, requestUserID uuid.UUID,
) error {
	return uc.contestRepo.DeleteContestTeam(ctx, contestID, teamID)
}

func (uc *ContestsUseCase) BlockProblemForUser(ctx context.Context, contestID, userID, problemID uuid.UUID, reason *string, operatorID uuid.UUID) error {
	return uc.transactions.Within(ctx, func(tx CommandTxStore) error {
		if err := tx.LockContest(ctx, contestID); err != nil {
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
			sub, err := tx.GetSubmission(ctx, change.ID)
			if err != nil {
				return err
			}
			if err := tx.Record(ctx, eventdata.Event{AggregateID: sub.ID, Type: eventdata.OutboxEventSubmissionCompleted, Payload: sub.CompletedEvent(change.PreviousState)}); err != nil {
				return err
			}
		}
		return ApplySubmissionResult(ctx, tx, &submissionsmodel.Submission{ContestID: &contestID, CreatedBy: &userID, ProblemID: &problemID})
	})
}

func (uc *ContestsUseCase) UnblockProblemForUser(ctx context.Context, contestID, userID, problemID uuid.UUID, rejudgeSubmissions bool) error {
	return uc.transactions.Within(ctx, func(tx CommandTxStore) error {
		if err := tx.LockContest(ctx, contestID); err != nil {
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
				sub, err := tx.GetSubmission(ctx, change.ID)
				if err != nil {
					return err
				}
				job, err := tx.ScheduleJob(ctx, sub.ID)
				if err != nil {
					return err
				}
				if err := tx.Record(ctx, eventdata.Event{AggregateID: sub.ID, Type: eventdata.OutboxEventSubmissionCreated, Payload: sub.CreatedEvent(job.ID, job.Generation, &change.PreviousState)}); err != nil {
					return err
				}
			}
		}
		return ApplySubmissionResult(ctx, tx, &submissionsmodel.Submission{ContestID: &contestID, CreatedBy: &userID, ProblemID: &problemID})
	})
}

func (uc *ContestsUseCase) GetProblemBlockStatusForUser(
	ctx context.Context,
	contestID, userID, problemID uuid.UUID,
) (*contestsmodel.ContestUserProblemBlock, error) {
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
	orgLogin, contestLogin string,
	userID uuid.UUID,
	message *string,
) (*contestsmodel.ContestJoinRequest, bool, error) {
	contest, err := uc.contestRepo.GetContestByOrgLoginAndContestLogin(ctx, orgLogin, contestLogin)
	if err != nil {
		return nil, false, err
	}

	// Check if already a member
	_, err = uc.contestRepo.GetContestMember(ctx, &contestsmodel.ContestPermissionGet{
		ContestId: contest.ID,
		UserId:    userID,
	})
	if err == nil {
		return nil, false, pkg.Wrap(pkg.ErrBadInput, nil, "вы уже являетесь участником этого контеста")
	}

	mode := contest.TypedSettings().GetParticipationMode()
	if mode == contestsmodel.ParticipationModeInviteOnly {
		return nil, false, pkg.Wrap(pkg.ErrBadInput, nil, "регистрация на данный контест закрыта (только по приглашениям)")
	}

	if mode == contestsmodel.ParticipationModeOpen {
		err := uc.CreateParticipant(ctx, contestsmodel.ParticipantCreation{
			ContestId: contest.ID,
			UserId:    userID,
		})
		if err != nil {
			return nil, false, err
		}
		return nil, true, nil
	}

	// Mode is ByRequest
	pending, err := uc.contestRepo.GetPendingContestJoinRequest(ctx, contest.ID, userID)
	if err != nil {
		return nil, false, err
	}
	if pending != nil {
		return nil, false, pkg.Wrap(pkg.ErrBadInput, nil, "заявка на участие уже отправлена и ожидает рассмотрения")
	}

	req, err := uc.contestRepo.CreateContestJoinRequest(ctx, &contestsmodel.CreateContestJoinRequestInput{
		ContestID: contest.ID,
		UserID:    userID,
		Message:   message,
	})
	if err != nil {
		return nil, false, err
	}

	// Send notifications to contest moderators / owner
	if uc.notificationsUC != nil && uc.usersRepo != nil {
		applicantUser, err := uc.usersRepo.GetUserById(ctx, userID)
		if err == nil {
			link := fmt.Sprintf("/%s/%s", orgLogin, contestLogin)
			if contest.OwnerID != nil {
				_, _ = uc.notificationsUC.Notify(ctx, &notificationsmodel.CreateNotificationInput{
					UserID: *contest.OwnerID,
					Type:   notificationsmodel.NotificationTypeContestJoinRequest,
					Title:  fmt.Sprintf("Новая заявка на участие в контесте %s", contest.Title),
					Body:   fmt.Sprintf("Пользователь @%s запросил доступ к контесту %s", applicantUser.Username, contest.Title),
					Link:   &link,
					Data: map[string]interface{}{
						"contest_id":         contest.ID.String(),
						"contest_title":      contest.Title,
						"contest_login":      contest.Login,
						"organization_login": orgLogin,
						"applicant_id":       userID.String(),
						"applicant_username": applicantUser.Username,
						"request_id":         req.ID.String(),
					},
				})
			}
		}
	}

	return req, false, nil
}

func (uc *ContestsUseCase) ListJoinRequests(
	ctx context.Context,
	orgLogin, contestLogin string,
	requestUserID uuid.UUID,
) ([]contestsmodel.ContestJoinRequest, error) {
	contest, err := uc.contestRepo.GetContestByOrgLoginAndContestLogin(ctx, orgLogin, contestLogin)
	if err != nil {
		return nil, err
	}

	// Verify user is moderator/owner or org admin
	member, err := uc.contestRepo.GetContestMember(ctx, &contestsmodel.ContestPermissionGet{
		ContestId: contest.ID,
		UserId:    requestUserID,
	})
	isModerator := err == nil && (member.ContestRole == contestsmodel.ContestRoleOwner || member.ContestRole == contestsmodel.ContestRoleModerator)

	if !isModerator && uc.orgsRepo != nil {
		orgMember, err := uc.orgsRepo.GetMember(ctx, contest.OrganizationID, requestUserID)
		if err == nil && (orgMember.Role == organizationsmodel.OrgRoleOwner || orgMember.Role == organizationsmodel.OrgRoleAdmin) {
			isModerator = true
		}
	}

	if !isModerator {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "access denied")
	}

	status := string(organizationsmodel.RequestStatusPending)
	return uc.contestRepo.ListContestJoinRequests(ctx, contest.ID, &status)
}

func (uc *ContestsUseCase) CancelJoinRequest(
	ctx context.Context,
	orgLogin, contestLogin string,
	requestUserID uuid.UUID,
) error {
	contest, err := uc.contestRepo.GetContestByOrgLoginAndContestLogin(ctx, orgLogin, contestLogin)
	if err != nil {
		return err
	}

	req, err := uc.contestRepo.GetPendingContestJoinRequest(ctx, contest.ID, requestUserID)
	if err != nil {
		return err
	}
	if req == nil {
		return pkg.Wrap(pkg.ErrNotFound, nil, "активная заявка не найдена")
	}

	return uc.contestRepo.UpdateContestJoinRequestStatus(ctx, req.ID, organizationsmodel.RequestStatusCanceled, nil)
}

func (uc *ContestsUseCase) ApproveJoinRequest(
	ctx context.Context,
	orgLogin, contestLogin string,
	requestID, reviewerID uuid.UUID,
) error {
	contest, err := uc.contestRepo.GetContestByOrgLoginAndContestLogin(ctx, orgLogin, contestLogin)
	if err != nil {
		return err
	}

	req, err := uc.contestRepo.GetContestJoinRequestByID(ctx, requestID)
	if err != nil {
		return err
	}
	if req.ContestID != contest.ID {
		return pkg.Wrap(pkg.ErrBadInput, nil, "request does not belong to this contest")
	}
	if req.Status != organizationsmodel.RequestStatusPending {
		return pkg.Wrap(pkg.ErrBadInput, nil, "заявка уже рассмотрена")
	}

	err = uc.transactions.Within(ctx, func(tx CommandTxStore) error {
		// Add member to contest
		err := tx.CreateContestMember(ctx, &contestsmodel.CreateContestMemberParams{
			ContestId: contest.ID,
			UserId:    req.UserID,
			Role:      contestsmodel.ContestRoleParticipant,
		})
		if err != nil {
			return fmt.Errorf("create contest member: %w", err)
		}

		if err := tx.UpdateContestJoinRequestStatus(ctx, req.ID, organizationsmodel.RequestStatusApproved, &reviewerID); err != nil {
			return fmt.Errorf("update join request status: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Notify applicant
	if uc.notificationsUC != nil {
		link := fmt.Sprintf("/%s/%s", orgLogin, contestLogin)
		_, _ = uc.notificationsUC.Notify(ctx, &notificationsmodel.CreateNotificationInput{
			UserID: req.UserID,
			Type:   notificationsmodel.NotificationTypeContestJoinApproved,
			Title:  fmt.Sprintf("Заявка на контест %s одобрена", contest.Title),
			Body:   fmt.Sprintf("Ваша заявка на участие в контесте %s была одобрена!", contest.Title),
			Link:   &link,
			Data: map[string]interface{}{
				"contest_id":         contest.ID.String(),
				"contest_title":      contest.Title,
				"contest_login":      contest.Login,
				"organization_login": orgLogin,
				"request_id":         req.ID.String(),
			},
		})
	}

	return nil
}

func (uc *ContestsUseCase) RejectJoinRequest(
	ctx context.Context,
	orgLogin, contestLogin string,
	requestID, reviewerID uuid.UUID,
) error {
	contest, err := uc.contestRepo.GetContestByOrgLoginAndContestLogin(ctx, orgLogin, contestLogin)
	if err != nil {
		return err
	}

	req, err := uc.contestRepo.GetContestJoinRequestByID(ctx, requestID)
	if err != nil {
		return err
	}
	if req.ContestID != contest.ID {
		return pkg.Wrap(pkg.ErrBadInput, nil, "request does not belong to this contest")
	}
	if req.Status != organizationsmodel.RequestStatusPending {
		return pkg.Wrap(pkg.ErrBadInput, nil, "заявка уже рассмотрена")
	}

	if err := uc.contestRepo.UpdateContestJoinRequestStatus(ctx, req.ID, organizationsmodel.RequestStatusRejected, &reviewerID); err != nil {
		return fmt.Errorf("update join request status: %w", err)
	}

	// Notify applicant
	if uc.notificationsUC != nil {
		link := fmt.Sprintf("/%s/%s", orgLogin, contestLogin)
		_, _ = uc.notificationsUC.Notify(ctx, &notificationsmodel.CreateNotificationInput{
			UserID: req.UserID,
			Type:   notificationsmodel.NotificationTypeContestJoinRejected,
			Title:  fmt.Sprintf("Заявка на контест %s отклонена", contest.Title),
			Body:   fmt.Sprintf("Ваша заявка на участие в контесте %s была отклонена.", contest.Title),
			Link:   &link,
			Data: map[string]interface{}{
				"contest_id":         contest.ID.String(),
				"contest_title":      contest.Title,
				"contest_login":      contest.Login,
				"organization_login": orgLogin,
				"request_id":         req.ID.String(),
			},
		})
	}

	return nil
}

func (uc *ContestsUseCase) GetMyPendingJoinRequest(
	ctx context.Context,
	orgLogin, contestLogin string,
	userID uuid.UUID,
) (*contestsmodel.ContestJoinRequest, error) {
	contest, err := uc.contestRepo.GetContestByOrgLoginAndContestLogin(ctx, orgLogin, contestLogin)
	if err != nil {
		return nil, err
	}
	return uc.contestRepo.GetPendingContestJoinRequest(ctx, contest.ID, userID)
}

func (uc *ContestsUseCase) scoreboardUserResults(ctx context.Context, contest contestsmodel.Contest, problems []contestsmodel.ContestProblem, userID uuid.UUID, frozen bool) ([]contestsmodel.ContestProblemResult, error) {
	return scoreboardUserResults(ctx, uc.contestRepo, contest, problems, userID, frozen)
}

func scoreboardItem(userID uuid.UUID, username string, pResults []contestsmodel.ContestProblemResult) contestsmodel.ScoreboardItem {
	var solvedCount int32
	var totalPenalty int32
	var lastAC *time.Time

	for _, r := range pResults {
		if r.Solved {
			solvedCount++
			var timeMins int32
			if r.TimeMinutes != nil {
				timeMins = *r.TimeMinutes
			}
			totalPenalty += timeMins + r.Penalty

			if r.FirstACTime != nil {
				if lastAC == nil || r.FirstACTime.After(*lastAC) {
					lastAC = r.FirstACTime
				}
			}
		}
	}

	return contestsmodel.ScoreboardItem{
		UserID:         userID,
		Username:       username,
		ProblemsSolved: solvedCount,
		TotalPenalty:   totalPenalty,
		LastAcceptedAt: lastAC,
		ProblemResults: pResults,
	}
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
	response := &contestsmodel.ScoreboardResponse{ContestID: contestID, ContestLogin: contest.Login, OrganizationLogin: contest.OrganizationLogin,
		IsFrozen: contest.IsFrozenAt(time.Now()), FreezeTime: contest.GetFreezeTime(), PenaltyPerAttempt: contest.GetPenaltyPerAttempt(),
		Items: []contestsmodel.ScoreboardItem{}, Problems: make([]contestsmodel.ScoreboardProblemHeader, len(problems))}
	for i, problem := range problems {
		response.Problems[i] = contestsmodel.ScoreboardProblemHeader{ProblemID: problem.ProblemID, Title: problem.Title, ShortName: problem.ShortName, Ordinal: int32(problem.Ordinal)} // #nosec G115
	}
	if member.ContestRole != contestsmodel.ContestRoleParticipant {
		return response, nil
	}
	results, err := uc.scoreboardUserResults(ctx, contest, problems, userID, response.IsFrozen && !unfrozen)
	if err != nil {
		return nil, err
	}
	response.Items = append(response.Items, scoreboardItem(userID, member.Username, results))
	return response, nil
}

func scoreboardUserResults(ctx context.Context, store interface {
	GetSubmissionsForScoreboard(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) ([]contestsmodel.SubmissionForScoreboard, error)
}, contest contestsmodel.Contest, problems []contestsmodel.ContestProblem, userID uuid.UUID, frozen bool) ([]contestsmodel.ContestProblemResult, error) {
	freezeTime := contest.GetFreezeTime()
	penaltyPerAttempt := contest.GetPenaltyPerAttempt()
	pResults := make([]contestsmodel.ContestProblemResult, 0, len(problems))

	for _, p := range problems {
		subs, err := store.GetSubmissionsForScoreboard(ctx, contest.ID, userID, p.ProblemID)
		if err != nil {
			return nil, err
		}

		var solved bool
		var failedAttempts int32
		var pendingAttempts int32
		var firstACTime *time.Time
		var timeMinutes *int32

		for _, sub := range subs {
			if contest.StartTime != nil && sub.CreatedAt.Before(*contest.StartTime) {
				continue
			}
			if contest.EndTime != nil && sub.CreatedAt.After(*contest.EndTime) {
				continue
			}

			if frozen && freezeTime != nil && !sub.CreatedAt.Before(*freezeTime) {
				pendingAttempts++
				continue
			}
			if frozen && freezeTime == nil && contest.GetFreezeStatus() == contestsmodel.FreezeStatusFrozen {
				pendingAttempts++
				continue
			}

			if sub.State == submissionsmodel.Saved || sub.State == submissionsmodel.GotCE {
				continue
			}

			if solved {
				continue
			}

			if sub.State == submissionsmodel.Accepted {
				solved = true
				t := sub.CreatedAt
				firstACTime = &t
				if contest.StartTime != nil {
					mins := int32(sub.CreatedAt.Sub(*contest.StartTime).Minutes())
					if mins < 0 {
						mins = 0
					}
					timeMinutes = &mins
				} else {
					zero := int32(0)
					timeMinutes = &zero
				}
			} else if sub.State == submissionsmodel.GotTL || sub.State == submissionsmodel.GotML ||
				sub.State == submissionsmodel.GotRE || sub.State == submissionsmodel.GotPE || sub.State == submissionsmodel.GotWA || sub.State == submissionsmodel.Disqualified {
				failedAttempts++
			}
		}

		if solved || failedAttempts > 0 || pendingAttempts > 0 || (!frozen && len(subs) > 0) {
			var penalty int32
			if solved {
				penalty = failedAttempts * penaltyPerAttempt
			}
			pResults = append(pResults, contestsmodel.ContestProblemResult{
				ContestID:       contest.ID,
				UserID:          userID,
				ProblemID:       p.ProblemID,
				Solved:          solved,
				FailedAttempts:  failedAttempts,
				PendingAttempts: pendingAttempts,
				FirstACTime:     firstACTime,
				TimeMinutes:     timeMinutes,
				Penalty:         penalty,
			})
		}
	}

	return pResults, nil
}

func ApplySubmissionResult(ctx context.Context, store ScoreboardStore, submission *submissionsmodel.Submission) error {
	if submission == nil || submission.ContestID == nil || submission.CreatedBy == nil || submission.ProblemID == nil {
		return nil
	}

	contestID := *submission.ContestID
	userID := *submission.CreatedBy
	problemID := *submission.ProblemID

	member, err := store.GetContestMember(ctx, &contestsmodel.ContestPermissionGet{
		ContestId: contestID,
		UserId:    userID,
	})
	if err != nil {
		if errors.Is(err, pkg.ErrNotFound) {
			return nil
		}
		return err
	}
	if member.ContestRole != contestsmodel.ContestRoleParticipant {
		return nil
	}

	contest, err := store.GetContest(ctx, contestID)
	if err != nil {
		return err
	}

	results, err := scoreboardUserResults(ctx, store, contest, []contestsmodel.ContestProblem{{ProblemID: problemID}}, userID, false)
	if err != nil {
		return err
	}
	var result contestsmodel.ContestProblemResult
	if len(results) > 0 {
		result = results[0]
	}
	return store.UpsertContestProblemResult(ctx, &contestsmodel.UpsertContestProblemResultParams{
		ContestID: contestID, UserID: userID, ProblemID: problemID,
		Solved: result.Solved, FailedAttempts: result.FailedAttempts,
		FirstACTime: result.FirstACTime, TimeMinutes: result.TimeMinutes,
	})
}
