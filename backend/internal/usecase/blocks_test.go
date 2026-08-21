package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/usecase"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type SubmissionsRepoMock struct {
	mock.Mock
	interfaces.SubmissionsRepo
}

func (m *SubmissionsRepoMock) GetSubmission(ctx context.Context, id uuid.UUID) (models.Submission, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(models.Submission), args.Error(1) //nolint:wrapcheck
}

func (m *SubmissionsRepoMock) CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error) {
	args := m.Called(ctx, creation)
	return args.Get(0).(uuid.UUID), args.Error(1) //nolint:wrapcheck
}

func (m *SubmissionsRepoMock) BlockSubmission(ctx context.Context, id uuid.UUID, reason *string) error {
	args := m.Called(ctx, id, reason)
	return args.Error(0) //nolint:wrapcheck
}

func (m *SubmissionsRepoMock) BlockUserProblemSubmissions(ctx context.Context, contestID, userID, problemID uuid.UUID, reason *string) error {
	args := m.Called(ctx, contestID, userID, problemID, reason)
	return args.Error(0) //nolint:wrapcheck
}

func (m *SubmissionsRepoMock) ResetSubmissionsState(ctx context.Context, filter models.RejudgeFilter) ([]uuid.UUID, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]uuid.UUID), args.Error(1) //nolint:wrapcheck
}

func (m *SubmissionsRepoMock) WithTx(tx pgx.Tx) interfaces.SubmissionsRepo {
	args := m.Called(tx)
	return args.Get(0).(interfaces.SubmissionsRepo)
}

type ContestsUCMock struct {
	mock.Mock
	interfaces.ContestsUC
}

func (m *ContestsUCMock) GetContest(ctx context.Context, id uuid.UUID) (models.Contest, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(models.Contest), args.Error(1) //nolint:wrapcheck
}

func (m *ContestsUCMock) GetContestProblem(ctx context.Context, c models.ContestProblemGet) (models.ContestProblem, error) {
	args := m.Called(ctx, c)
	return args.Get(0).(models.ContestProblem), args.Error(1) //nolint:wrapcheck
}

func (m *ContestsUCMock) ProcessSubmissionResult(ctx context.Context, submission *models.Submission) error {
	args := m.Called(ctx, submission)
	return args.Error(0) //nolint:wrapcheck
}

func (m *ContestsUCMock) GetProblemBlockStatusForUser(ctx context.Context, contestID, userID, problemID uuid.UUID) (*models.ContestUserProblemBlock, error) {
	args := m.Called(ctx, contestID, userID, problemID)
	if res := args.Get(0); res != nil {
		return res.(*models.ContestUserProblemBlock), args.Error(1) //nolint:wrapcheck
	}
	return nil, args.Error(1) //nolint:wrapcheck
}

type ProblemsUCMock struct {
	mock.Mock
	interfaces.ProblemsUC
}

func (m *ProblemsUCMock) GetProblemById(ctx context.Context, id uuid.UUID) (models.Problem, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(models.Problem), args.Error(1) //nolint:wrapcheck
}

type OutboxRepoMock struct {
	mock.Mock
	interfaces.OutboxRepo
}

func (m *OutboxRepoMock) CreateEvent(ctx context.Context, params *models.CreateOutboxEventParams) error {
	args := m.Called(ctx, params)
	return args.Error(0) //nolint:wrapcheck
}

func (m *OutboxRepoMock) WithTx(tx pgx.Tx) interfaces.OutboxRepo {
	args := m.Called(tx)
	return args.Get(0).(interfaces.OutboxRepo)
}

type TransactorMock struct {
	mock.Mock
	interfaces.Transactor
}

func (m *TransactorMock) WithTx(ctx context.Context, fn func(context.Context, pgx.Tx) error) error {
	args := m.Called(ctx, fn)
	if args.Get(0) != nil {
		return args.Error(0) //nolint:wrapcheck
	}
	return fn(ctx, nil)
}

func TestSubmissionsUseCase_BlockSubmission(t *testing.T) {
	ctx := context.Background()
	contestID := uuid.New()
	submissionID := uuid.New()
	userID := uuid.New()
	problemID := uuid.New()
	reason := "Cheating on test 1"

	subRepo := new(SubmissionsRepoMock)
	contestsUC := new(ContestsUCMock)
	problemsUC := new(ProblemsUCMock)
	outboxRepo := new(OutboxRepoMock)
	txManager := new(TransactorMock)

	uc := usecase.NewSubmissionsUseCase(subRepo, contestsUC, problemsUC, outboxRepo, txManager)

	sub := models.Submission{
		ID:        submissionID,
		ContestID: &contestID,
		CreatedBy: &userID,
		ProblemID: &problemID,
		State:     models.Accepted,
	}

	subRepo.On("GetSubmission", ctx, submissionID).Return(sub, nil)
	subRepo.On("BlockSubmission", ctx, submissionID, &reason).Return(nil)

	expectedSub := sub
	expectedSub.State = models.Disqualified
	expectedSub.BanReason = &reason
	contestsUC.On("ProcessSubmissionResult", ctx, &expectedSub).Return(nil)

	err := uc.BlockSubmission(ctx, contestID, submissionID, &reason)
	require.NoError(t, err)
	subRepo.AssertExpectations(t)
	contestsUC.AssertExpectations(t)
}

func TestSubmissionsUseCase_UnblockSubmission(t *testing.T) {
	ctx := context.Background()
	contestID := uuid.New()
	submissionID := uuid.New()

	subRepo := new(SubmissionsRepoMock)
	contestsUC := new(ContestsUCMock)
	problemsUC := new(ProblemsUCMock)
	outboxRepo := new(OutboxRepoMock)
	txManager := new(TransactorMock)

	uc := usecase.NewSubmissionsUseCase(subRepo, contestsUC, problemsUC, outboxRepo, txManager)

	contestsUC.On("GetContest", ctx, contestID).Return(models.Contest{ID: contestID}, nil)
	subRepo.On("GetSubmission", ctx, submissionID).Return(models.Submission{
		ID:        submissionID,
		ContestID: &contestID,
	}, nil)

	txSubRepo := new(SubmissionsRepoMock)
	txOutboxRepo := new(OutboxRepoMock)
	subRepo.On("WithTx", mock.Anything).Return(txSubRepo)
	outboxRepo.On("WithTx", mock.Anything).Return(txOutboxRepo)
	txManager.On("WithTx", ctx, mock.Anything).Return(nil)

	rejudgeFilter := models.RejudgeFilter{
		ContestID:    contestID,
		SubmissionID: &submissionID,
	}
	txSubRepo.On("ResetSubmissionsState", ctx, rejudgeFilter).Return([]uuid.UUID{submissionID}, nil)
	txSubRepo.On("GetSubmission", ctx, submissionID).Return(models.Submission{
		ID:        submissionID,
		ContestID: &contestID,
		State:     models.Saved,
		CreatedAt: time.Now(),
	}, nil)
	txOutboxRepo.On("CreateEvent", ctx, mock.Anything).Return(nil)

	err := uc.UnblockSubmission(ctx, contestID, submissionID)
	require.NoError(t, err)
}

func TestContestsUseCase_BlockProblemForUser(t *testing.T) {
	ctx := context.Background()
	contestID := uuid.New()
	userID := uuid.New()
	problemID := uuid.New()
	operatorID := uuid.New()
	reason := "Banned from problem A"

	mockRepo := new(ContestsRepoMock)
	mockSubRepo := new(SubmissionsRepoMock)
	uc := usecase.NewContestsUseCase(mockRepo, nil, mockSubRepo)

	mockRepo.On("GetContest", mock.Anything, contestID).Return(models.Contest{ID: contestID}, nil)
	mockRepo.On("CreateContestUserProblemBlock", mock.Anything, &models.CreateContestUserProblemBlockParams{
		ContestID: contestID,
		UserID:    userID,
		ProblemID: problemID,
		Reason:    &reason,
		CreatedBy: &operatorID,
	}).Return(nil)

	mockSubRepo.On("BlockUserProblemSubmissions", ctx, contestID, userID, problemID, &reason).Return(nil)

	mockRepo.On("GetContestMember", mock.Anything, &models.ContestPermissionGet{
		ContestId: contestID,
		UserId:    userID,
	}).Return(models.ContestMember{
		ContestID:   contestID,
		UserID:      userID,
		ContestRole: models.ContestRoleParticipant,
	}, nil)

	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userID, problemID).
		Return([]models.SubmissionForScoreboard{
			{State: models.Disqualified, CreatedAt: time.Now()},
		}, nil)

	mockRepo.On("UpsertContestProblemResult", mock.Anything, &models.UpsertContestProblemResultParams{
		ContestID:      contestID,
		UserID:         userID,
		ProblemID:      problemID,
		Solved:         false,
		FailedAttempts: 1,
		FirstACTime:    nil,
		TimeMinutes:    nil,
	}).Return(nil)

	err := uc.BlockProblemForUser(ctx, contestID, userID, problemID, &reason, operatorID)
	require.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockSubRepo.AssertExpectations(t)
}

func TestContestsUseCase_UnblockProblemForUser(t *testing.T) {
	ctx := context.Background()
	contestID := uuid.New()
	userID := uuid.New()
	problemID := uuid.New()

	mockRepo := new(ContestsRepoMock)
	mockSubRepo := new(SubmissionsRepoMock)
	uc := usecase.NewContestsUseCase(mockRepo, nil, mockSubRepo)

	mockRepo.On("GetContest", mock.Anything, contestID).Return(models.Contest{ID: contestID}, nil)
	mockRepo.On("DeleteContestUserProblemBlock", mock.Anything, contestID, userID, problemID).Return(nil)

	mockRepo.On("GetContestMember", mock.Anything, &models.ContestPermissionGet{
		ContestId: contestID,
		UserId:    userID,
	}).Return(models.ContestMember{
		ContestID:   contestID,
		UserID:      userID,
		ContestRole: models.ContestRoleParticipant,
	}, nil)

	mockRepo.On("GetSubmissionsForScoreboard", mock.Anything, contestID, userID, problemID).
		Return([]models.SubmissionForScoreboard{}, nil)

	mockRepo.On("UpsertContestProblemResult", mock.Anything, &models.UpsertContestProblemResultParams{
		ContestID:      contestID,
		UserID:         userID,
		ProblemID:      problemID,
		Solved:         false,
		FailedAttempts: 0,
		FirstACTime:    nil,
		TimeMinutes:    nil,
	}).Return(nil)

	err := uc.UnblockProblemForUser(ctx, contestID, userID, problemID, false)
	require.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestSubmissionsUseCase_CreateSubmission_WhenProblemBlocked(t *testing.T) {
	ctx := context.Background()
	contestID := uuid.New()
	userID := uuid.New()
	problemID := uuid.New()
	submissionID := uuid.New()
	reason := "Banned from submitting to problem"

	subRepo := new(SubmissionsRepoMock)
	contestsUC := new(ContestsUCMock)
	problemsUC := new(ProblemsUCMock)
	outboxRepo := new(OutboxRepoMock)
	txManager := new(TransactorMock)

	uc := usecase.NewSubmissionsUseCase(subRepo, contestsUC, problemsUC, outboxRepo, txManager)

	contestsUC.On("GetContest", ctx, contestID).Return(models.Contest{ID: contestID}, nil)
	problemsUC.On("GetProblemById", ctx, problemID).Return(models.Problem{ID: problemID}, nil)
	contestsUC.On("GetContestProblem", ctx, models.ContestProblemGet{
		ContestId: contestID,
		ProblemId: problemID,
	}).Return(models.ContestProblem{ContestID: contestID, ProblemID: problemID}, nil)

	// Block exists for this problem and user!
	contestsUC.On("GetProblemBlockStatusForUser", ctx, contestID, userID, problemID).Return(&models.ContestUserProblemBlock{
		ContestID: contestID,
		UserID:    userID,
		ProblemID: problemID,
		Reason:    &reason,
	}, nil)

	dqState := models.Disqualified
	creation := &models.SubmissionCreation{
		UserId:    userID,
		ContestId: contestID,
		ProblemId: problemID,
		Solution:  "int main(){}",
		Language:  models.Cpp,
		Penalty:   20,
	}

	expectedCreation := *creation
	expectedCreation.State = &dqState
	expectedCreation.BanReason = &reason

	subRepo.On("CreateSubmission", ctx, &expectedCreation).Return(submissionID, nil)
	subRepo.On("GetSubmission", ctx, submissionID).Return(models.Submission{
		ID:        submissionID,
		ContestID: &contestID,
		CreatedBy: &userID,
		ProblemID: &problemID,
		State:     models.Disqualified,
		BanReason: &reason,
	}, nil)

	contestsUC.On("ProcessSubmissionResult", ctx, mock.MatchedBy(func(sub *models.Submission) bool {
		return sub != nil && sub.ID == submissionID && sub.State == models.Disqualified
	})).Return(nil)

	id, err := uc.CreateSubmission(ctx, creation)
	require.NoError(t, err)
	assert.Equal(t, submissionID, id)
}
