package submissions

import (
	"context"
	"testing"
	"time"

	"github.com/gate149/core/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockRepo struct {
	mock.Mock
}

func (m *MockRepo) GetSubmissions(ctx context.Context, id uuid.UUID) (*models.Submission, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Submission), args.Error(1)
}

func (m *MockRepo) CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error) {
	args := m.Called(ctx, creation)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockRepo) UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error {
	args := m.Called(ctx, id, update)
	return args.Error(0)
}

func (m *MockRepo) ListSolutions(ctx context.Context, filter models.SolutionsFilter) (*models.SolutionsList, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SolutionsList), args.Error(1)
}

type MockProblemsUC struct {
	mock.Mock
}

func (m *MockProblemsUC) GetProblemById(ctx context.Context, id uuid.UUID) (*models.Problem, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Problem), args.Error(1)
}

func (m *MockProblemsUC) DownloadTestsArchive(ctx context.Context, id uuid.UUID) (string, error) {
	args := m.Called(ctx, id)
	return args.String(0), args.Error(1)
}

func (m *MockProblemsUC) UnarchiveTestsArchive(ctx context.Context, zipPath, destDirPath string) (string, error) {
	args := m.Called(ctx, zipPath, destDirPath)
	return args.String(0), args.Error(1)
}

type MockPublisher struct {
	mock.Mock
}

func (m *MockPublisher) Publish(subject string, data []byte) error {
	args := m.Called(subject, data)
	return args.Error(0)
}

func TestUseCase_GetSolution(t *testing.T) {
	mockRepo := new(MockRepo)
	mockProblemsUC := new(MockProblemsUC)
	mockPub := new(MockPublisher)

	uc := NewUseCase(mockRepo, mockProblemsUC, mockPub, nil, "test-queue")
	ctx := context.Background()
	id := uuid.New()

	expectedSolution := &models.Submission{
		Id:       id,
		UserId:   uuid.New(),
		Solution: "test solution",
		State:    models.Saved,
	}

	mockRepo.On("GetSubmissions", ctx, id).Return(expectedSolution, nil)

	solution, err := uc.GetSubmissions(ctx, id)
	assert.NoError(t, err)
	assert.Equal(t, expectedSolution, solution)
	mockRepo.AssertExpectations(t)
}

func TestUseCase_CreateSolution(t *testing.T) {
	mockRepo := new(MockRepo)
	mockProblemsUC := new(MockProblemsUC)
	mockPub := new(MockPublisher)

	uc := NewUseCase(mockRepo, mockProblemsUC, mockPub, nil, "test-queue")
	ctx := context.Background()

	problemID := uuid.New()
	creation := &models.SubmissionCreation{
		UserId:    uuid.New(),
		ProblemId: problemID,
		ContestId: uuid.New(),
		Language:  models.Cpp,
		Solution:  "int main() { return 0; }",
		Penalty:   20,
	}

	expectedID := uuid.New()
	mockRepo.On("CreateSubmission", ctx, creation).Return(expectedID, nil)

	// Mock the goroutine calls
	mockProblemsUC.On("GetProblemById", mock.Anything, problemID).Return(&models.Problem{
		Id:    problemID,
		Title: "Test Problem",
		Meta: models.Meta{
			Count: 0, // No tests, will result in immediate acceptance
		},
	}, nil)
	mockRepo.On("UpdateSubmission", mock.Anything, expectedID, mock.AnythingOfType("*models.SubmissionUpdate")).Return(nil)

	id, err := uc.CreateSubmission(ctx, creation)
	assert.NoError(t, err)
	assert.Equal(t, expectedID, id)

	// Wait for goroutine to complete
	time.Sleep(100 * time.Millisecond)

	mockRepo.AssertExpectations(t)
	mockProblemsUC.AssertExpectations(t)
}

func TestUseCase_UpdateSolution(t *testing.T) {
	mockRepo := new(MockRepo)
	mockProblemsUC := new(MockProblemsUC)
	mockPub := new(MockPublisher)

	uc := NewUseCase(mockRepo, mockProblemsUC, mockPub, nil, "test-queue")
	ctx := context.Background()

	id := uuid.New()
	update := &models.SubmissionUpdate{
		State: models.Accepted,
		Score: 100,
	}

	mockRepo.On("UpdateSubmission", ctx, id, update).Return(nil)

	err := uc.UpdateSubmission(ctx, id, update)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestUseCase_ListSolutions(t *testing.T) {
	mockRepo := new(MockRepo)
	mockProblemsUC := new(MockProblemsUC)
	mockPub := new(MockPublisher)

	uc := NewUseCase(mockRepo, mockProblemsUC, mockPub, nil, "test-queue")
	ctx := context.Background()

	contestID := uuid.New()
	filter := models.SolutionsFilter{
		Page:      1,
		PageSize:  10,
		ContestId: &contestID,
	}

	expectedList := &models.SolutionsList{
		Solutions: []*models.SubmissionListItem{
			{
				Id:     uuid.New(),
				UserId: uuid.New(),
				State:  models.Accepted,
			},
		},
		Pagination: models.Pagination{
			Page:  1,
			Total: 1,
		},
	}

	mockRepo.On("ListSolutions", ctx, filter).Return(expectedList, nil)

	list, err := uc.ListSolutions(ctx, filter)
	assert.NoError(t, err)
	assert.Equal(t, expectedList, list)
	assert.Equal(t, 1, len(list.Solutions))
	mockRepo.AssertExpectations(t)
}
