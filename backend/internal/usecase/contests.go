package usecase

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type ContestsUseCase struct {
	contestRepo     interfaces.ContestsRepo
	orgsRepo        interfaces.OrganizationsRepo
	submissionsRepo interfaces.SubmissionsRepo
}

func NewContestsUseCase(
	contestRepo interfaces.ContestsRepo,
	orgsRepo interfaces.OrganizationsRepo,
	submissionsRepo ...interfaces.SubmissionsRepo,
) *ContestsUseCase {
	var subRepo interfaces.SubmissionsRepo
	if len(submissionsRepo) > 0 {
		subRepo = submissionsRepo[0]
	}
	return &ContestsUseCase{
		contestRepo:     contestRepo,
		orgsRepo:        orgsRepo,
		submissionsRepo: subRepo,
	}
}

func (uc *ContestsUseCase) CreateContest(
	ctx context.Context,
	c *models.CreateContestInput,
) (uuid.UUID, error) {
	params := &models.CreateContestParams{
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

	err := uc.contestRepo.CreateContest(ctx, params)
	if err != nil {
		return uuid.Nil, pkg.Wrap(err, nil, "can't create contest")
	}

	// If an owner ID was provided, add them as a contest member with owner role
	if c.OwnerID != nil {
		err = uc.contestRepo.CreateContestMember(ctx, &models.CreateContestMemberParams{
			ContestId: params.ID,
			UserId:    *c.OwnerID,
			Role:      models.ContestRoleOwner,
		})
		if err != nil {
			return uuid.Nil, pkg.Wrap(err, nil, "can't create contest member")
		}
	}

	return params.ID, nil
}

func (uc *ContestsUseCase) GetContest(ctx context.Context, id uuid.UUID) (models.Contest, error) {
	contest, err := uc.contestRepo.GetContest(ctx, id)
	if err != nil {
		return models.Contest{}, err
	}
	return contest, nil
}

func (uc *ContestsUseCase) GetContestByOrgLoginAndContestLogin(ctx context.Context, orgLogin, contestLogin string) (models.Contest, error) {
	contest, err := uc.contestRepo.GetContestByOrgLoginAndContestLogin(ctx, orgLogin, contestLogin)
	if err != nil {
		return models.Contest{}, err
	}
	return contest, nil
}

func (uc *ContestsUseCase) ListOrganizationContests(ctx context.Context, orgID uuid.UUID, search string, visibility string, page, pageSize int32) (*models.ContestsList, error) {
	contests, total, err := uc.contestRepo.ListOrganizationContests(ctx, orgID, search, visibility, page, pageSize)
	if err != nil {
		return nil, pkg.Wrap(err, nil, "can't list organization contests from database")
	}

	return &models.ContestsList{
		Contests:   contests,
		Pagination: models.NewPagination(page, pageSize, total),
	}, nil
}

func (uc *ContestsUseCase) ListAdminContests(ctx context.Context, filter models.AdminContestsFilter) (*models.ContestsList, error) {
	contests, total, err := uc.contestRepo.ListAdminContests(ctx, filter)
	if err != nil {
		return nil, pkg.Wrap(err, nil, "can't list admin contests from database")
	}

	return &models.ContestsList{
		Contests:   contests,
		Pagination: models.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

func (uc *ContestsUseCase) ListUserContests(ctx context.Context, filter models.UserContestsFilter) (*models.ContestsList, error) {
	contests, total, err := uc.contestRepo.ListUserContests(ctx, filter)
	if err != nil {
		return nil, pkg.Wrap(err, nil, "can't list user contests from database")
	}

	return &models.ContestsList{
		Contests:   contests,
		Pagination: models.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

func (uc *ContestsUseCase) ListWorkshopContests(ctx context.Context, filter models.WorkshopContestsFilter) (*models.ContestsList, error) {
	contests, total, err := uc.contestRepo.ListWorkshopContests(ctx, filter)
	if err != nil {
		return nil, pkg.Wrap(err, nil, "can't list workshop contests from database")
	}

	return &models.ContestsList{
		Contests:   contests,
		Pagination: models.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

func (uc *ContestsUseCase) ListPublicContests(ctx context.Context, filter models.PublicContestsFilter) (*models.ContestsList, error) {
	contests, total, err := uc.contestRepo.ListPublicContests(ctx, filter)
	if err != nil {
		return nil, pkg.Wrap(err, nil, "can't list public contests from database")
	}

	return &models.ContestsList{
		Contests:   contests,
		Pagination: models.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

func (uc *ContestsUseCase) UpdateContest(ctx context.Context, c models.ContestUpdateInput) error {
	params := models.ContestUpdateParams(c)

	return uc.contestRepo.UpdateContest(ctx, params)
}

func (uc *ContestsUseCase) DeleteContest(ctx context.Context, id uuid.UUID) error {
	return uc.contestRepo.DeleteContest(ctx, id)
}

func (uc *ContestsUseCase) CreateContestProblem(ctx context.Context, c models.ContestProblemCreation) error {
	return uc.contestRepo.CreateContestProblem(ctx, c)
}

func (uc *ContestsUseCase) GetContestProblem(ctx context.Context, c models.ContestProblemGet) (models.ContestProblem, error) {
	return uc.contestRepo.GetContestProblem(ctx, c)
}

func (uc *ContestsUseCase) GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]models.ContestProblem, error) {
	return uc.contestRepo.GetContestProblems(ctx, contestId)
}

func (uc *ContestsUseCase) UpdateContestProblemPackage(ctx context.Context, contestId, problemId, packageId uuid.UUID) error {
	return uc.contestRepo.UpdateContestProblemPackage(ctx, contestId, problemId, packageId)
}

func (uc *ContestsUseCase) DeleteContestProblem(ctx context.Context, c models.ContestProblemDeletion) error {
	return uc.contestRepo.DeleteContestProblem(ctx, c)
}

func (uc *ContestsUseCase) ReorderContestProblems(ctx context.Context, contestId uuid.UUID, problems []models.ContestProblemReorderItem) error {
	return uc.contestRepo.ReorderContestProblems(ctx, contestId, problems)
}

func (uc *ContestsUseCase) CreateParticipant(ctx context.Context, c models.ParticipantCreation) error {
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

	return uc.contestRepo.CreateContestMember(ctx, &models.CreateContestMemberParams{
		ContestId: c.ContestId,
		UserId:    c.UserId,
		Role:      models.ContestRoleParticipant,
	})
}

func (uc *ContestsUseCase) DeleteParticipant(ctx context.Context, c models.ParticipantDeletion) error {
	return uc.contestRepo.DeleteContestMember(ctx, c.UserId, c.ContestId)
}

func (uc *ContestsUseCase) ListParticipants(ctx context.Context, filter models.ParticipantsFilter) (*models.ContestMembersList, error) {
	members, total, err := uc.contestRepo.ListContestMembers(ctx, filter)
	if err != nil {
		return nil, err
	}

	return &models.ContestMembersList{
		Members:    members,
		Pagination: models.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

func (uc *ContestsUseCase) GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (models.ContestMember, error) {
	return uc.contestRepo.GetContestMember(ctx, c)
}

func (uc *ContestsUseCase) UpdateContestMember(ctx context.Context, contestId uuid.UUID, userId uuid.UUID, role string) error {
	if role != models.ContestRoleOwner && role != models.ContestRoleModerator && role != models.ContestRoleParticipant {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid role value")
	}

	currentMember, err := uc.GetContestMember(ctx, &models.ContestPermissionGet{
		ContestId: contestId,
		UserId:    userId,
	})
	if err != nil {
		return pkg.Wrap(err, nil, "can't get contest member")
	}

	if currentMember.Role == models.ContestRoleOwner {
		return pkg.Wrap(pkg.ErrBadInput, nil, "cannot change role from owner")
	}

	if role == models.ContestRoleOwner {
		return pkg.Wrap(pkg.ErrBadInput, nil, "cannot change role to owner")
	}

	err = uc.contestRepo.UpdateContestMember(ctx, contestId, userId, role)
	if err != nil {
		return pkg.Wrap(err, nil, "can't update contest member")
	}

	return nil
}

func (uc *ContestsUseCase) ListDashboardContests(ctx context.Context, userID uuid.UUID, limit int32) ([]models.DashboardContest, error) {
	contests, err := uc.contestRepo.ListDashboardContests(ctx, userID, limit)
	if err != nil {
		return nil, pkg.Wrap(err, nil, "can't list dashboard contests")
	}
	return contests, nil
}

func (uc *ContestsUseCase) ProcessSubmissionResult(ctx context.Context, submission *models.Submission) error {
	if submission == nil || submission.ContestID == nil || submission.CreatedBy == nil || submission.ProblemID == nil {
		return nil
	}

	contestID := *submission.ContestID
	userID := *submission.CreatedBy
	problemID := *submission.ProblemID

	member, err := uc.contestRepo.GetContestMember(ctx, &models.ContestPermissionGet{
		ContestId: contestID,
		UserId:    userID,
	})
	if err != nil {
		return nil //nolint:nilerr // Non-members do not participate in scoreboard
	}
	if member.ContestRole != models.ContestRoleParticipant {
		return nil
	}

	contest, err := uc.contestRepo.GetContest(ctx, contestID)
	if err != nil {
		return err
	}

	submissions, err := uc.contestRepo.GetSubmissionsForScoreboard(ctx, contestID, userID, problemID)
	if err != nil {
		return err
	}

	var solved bool
	var failedAttempts int32
	var firstACTime *time.Time
	var timeMinutes *int32

	for _, sub := range submissions {
		if contest.StartTime != nil && sub.CreatedAt.Before(*contest.StartTime) {
			continue
		}
		if contest.EndTime != nil && sub.CreatedAt.After(*contest.EndTime) {
			continue
		}

		if sub.State == models.Saved || sub.State == models.GotCE {
			continue
		}

		if sub.State == models.Accepted {
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
			break
		} else if sub.State == models.GotTL || sub.State == models.GotML ||
			sub.State == models.GotRE || sub.State == models.GotPE || sub.State == models.GotWA || sub.State == models.Disqualified {
			failedAttempts++
		}
	}

	return uc.contestRepo.UpsertContestProblemResult(ctx, &models.UpsertContestProblemResultParams{
		ContestID:      contestID,
		UserID:         userID,
		ProblemID:      problemID,
		Solved:         solved,
		FailedAttempts: failedAttempts,
		FirstACTime:    firstACTime,
		TimeMinutes:    timeMinutes,
	})
}

func (uc *ContestsUseCase) GetContestScoreboard(ctx context.Context, contestID, userID uuid.UUID, unfrozen bool) (*models.ScoreboardResponse, error) {
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

	problemHeaders := make([]models.ScoreboardProblemHeader, len(problems))
	for i, p := range problems {
		problemHeaders[i] = models.ScoreboardProblemHeader{
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

		userResults := make(map[uuid.UUID][]models.ContestProblemResult)
		for _, res := range results {
			res.Penalty = 0
			if res.Solved {
				res.Penalty = res.FailedAttempts * penaltyPerAttempt
			}
			res.PendingAttempts = 0
			userResults[res.UserID] = append(userResults[res.UserID], res)
		}

		items := make([]models.ScoreboardItem, 0, len(userMap))
		for uID, username := range userMap {
			pResults := userResults[uID]
			if pResults == nil {
				pResults = []models.ContestProblemResult{}
			}

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

			items = append(items, models.ScoreboardItem{
				UserID:         uID,
				Username:       username,
				ProblemsSolved: solvedCount,
				TotalPenalty:   totalPenalty,
				LastAcceptedAt: lastAC,
				ProblemResults: pResults,
			})
		}

		sortScoreboardItems(items)

		return &models.ScoreboardResponse{
			ContestID:         contestID,
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

	items := make([]models.ScoreboardItem, 0, len(userMap))
	for uID, username := range userMap {
		var pResults []models.ContestProblemResult

		for _, p := range problems {
			subs, err := uc.contestRepo.GetSubmissionsForScoreboard(ctx, contestID, uID, p.ProblemID)
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

				if sub.State == models.Saved || sub.State == models.GotCE {
					continue
				}

				if freezeTime != nil && !sub.CreatedAt.Before(*freezeTime) {
					pendingAttempts++
					continue
				}
				if freezeTime == nil && contest.GetFreezeStatus() == models.FreezeStatusFrozen {
					pendingAttempts++
					continue
				}

				if solved {
					continue
				}

				if sub.State == models.Accepted {
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
				} else if sub.State == models.GotTL || sub.State == models.GotML ||
					sub.State == models.GotRE || sub.State == models.GotPE || sub.State == models.GotWA || sub.State == models.Disqualified {
					failedAttempts++
				}
			}

			if solved || failedAttempts > 0 || pendingAttempts > 0 {
				var penalty int32
				if solved {
					penalty = failedAttempts * penaltyPerAttempt
				}
				pResults = append(pResults, models.ContestProblemResult{
					ContestID:       contestID,
					UserID:          uID,
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

		items = append(items, models.ScoreboardItem{
			UserID:         uID,
			Username:       username,
			ProblemsSolved: solvedCount,
			TotalPenalty:   totalPenalty,
			LastAcceptedAt: lastAC,
			ProblemResults: pResults,
		})
	}

	sortScoreboardItems(items)

	return &models.ScoreboardResponse{
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

func sortScoreboardItems(items []models.ScoreboardItem) {
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
	role models.ContestRole,
) error {
	return uc.contestRepo.CreateContestTeam(ctx, contestID, teamID, role)
}

func (uc *ContestsUseCase) GetContestTeams(
	ctx context.Context,
	contestID, requestUserID uuid.UUID,
) ([]models.ContestTeam, error) {
	return uc.contestRepo.GetContestTeams(ctx, contestID)
}

func (uc *ContestsUseCase) UpdateContestTeamRole(
	ctx context.Context,
	contestID, teamID, requestUserID uuid.UUID,
	role models.ContestRole,
) error {
	return uc.contestRepo.UpdateContestTeamRole(ctx, contestID, teamID, role)
}

func (uc *ContestsUseCase) DeleteContestTeam(
	ctx context.Context,
	contestID, teamID, requestUserID uuid.UUID,
) error {
	return uc.contestRepo.DeleteContestTeam(ctx, contestID, teamID)
}

func (uc *ContestsUseCase) BlockProblemForUser(
	ctx context.Context,
	contestID, userID, problemID uuid.UUID,
	reason *string,
	operatorID uuid.UUID,
) error {
	_, err := uc.GetContest(ctx, contestID)
	if err != nil {
		return err
	}

	err = uc.contestRepo.CreateContestUserProblemBlock(ctx, &models.CreateContestUserProblemBlockParams{
		ContestID: contestID,
		UserID:    userID,
		ProblemID: problemID,
		Reason:    reason,
		CreatedBy: &operatorID,
	})
	if err != nil {
		return err
	}

	if uc.submissionsRepo != nil {
		err = uc.submissionsRepo.BlockUserProblemSubmissions(ctx, contestID, userID, problemID, reason)
		if err != nil {
			return err
		}
	}

	dummySub := &models.Submission{
		ContestID: &contestID,
		CreatedBy: &userID,
		ProblemID: &problemID,
	}
	_ = uc.ProcessSubmissionResult(ctx, dummySub)

	return nil
}

func (uc *ContestsUseCase) UnblockProblemForUser(
	ctx context.Context,
	contestID, userID, problemID uuid.UUID,
	rejudgeSubmissions bool,
) error {
	_, err := uc.GetContest(ctx, contestID)
	if err != nil {
		return err
	}

	err = uc.contestRepo.DeleteContestUserProblemBlock(ctx, contestID, userID, problemID)
	if err != nil {
		return err
	}

	dummySub := &models.Submission{
		ContestID: &contestID,
		CreatedBy: &userID,
		ProblemID: &problemID,
	}
	_ = uc.ProcessSubmissionResult(ctx, dummySub)

	return nil
}

func (uc *ContestsUseCase) GetProblemBlockStatusForUser(
	ctx context.Context,
	contestID, userID, problemID uuid.UUID,
) (*models.ContestUserProblemBlock, error) {
	block, err := uc.contestRepo.GetContestUserProblemBlock(ctx, contestID, userID, problemID)
	if err != nil {
		if errors.Is(err, pkg.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return block, nil
}

