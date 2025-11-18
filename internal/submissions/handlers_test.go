package submissions

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	testerv1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/models"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	ory "github.com/ory/client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockSolutionsUC struct {
	mock.Mock
}

func (m *MockSolutionsUC) GetSubmissions(ctx context.Context, id uuid.UUID) (*models.Submission, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Submission), args.Error(1)
}

func (m *MockSolutionsUC) CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error) {
	args := m.Called(ctx, creation)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockSolutionsUC) UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error {
	args := m.Called(ctx, id, update)
	return args.Error(0)
}

func (m *MockSolutionsUC) ListSolutions(ctx context.Context, filter models.SolutionsFilter) (*models.SolutionsList, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SolutionsList), args.Error(1)
}

type MockContestsUC struct {
	mock.Mock
}

func (m *MockContestsUC) CreateContest(ctx context.Context, creation models.ContestCreation) (uuid.UUID, error) {
	args := m.Called(ctx, creation)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockContestsUC) GetContest(ctx context.Context, id uuid.UUID) (*models.Contest, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Contest), args.Error(1)
}

func (m *MockContestsUC) ListContests(ctx context.Context, filter models.ContestsFilter) (*models.ContestsList, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ContestsList), args.Error(1)
}

func (m *MockContestsUC) UpdateContest(ctx context.Context, id uuid.UUID, update models.ContestUpdate) error {
	args := m.Called(ctx, id, update)
	return args.Error(0)
}

func (m *MockContestsUC) DeleteContest(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockContestsUC) CreateContestProblem(ctx context.Context, contestId, problemId uuid.UUID) error {
	args := m.Called(ctx, contestId, problemId)
	return args.Error(0)
}

func (m *MockContestsUC) GetContestProblem(ctx context.Context, contestId, problemId uuid.UUID) (*models.ContestProblem, error) {
	args := m.Called(ctx, contestId, problemId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ContestProblem), args.Error(1)
}

func (m *MockContestsUC) GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]*models.ContestProblemsListItem, error) {
	args := m.Called(ctx, contestId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.ContestProblemsListItem), args.Error(1)
}

func (m *MockContestsUC) DeleteContestProblem(ctx context.Context, contestId, problemId uuid.UUID) error {
	args := m.Called(ctx, contestId, problemId)
	return args.Error(0)
}

func (m *MockContestsUC) CreateParticipant(ctx context.Context, contestId, userId uuid.UUID) error {
	args := m.Called(ctx, contestId, userId)
	return args.Error(0)
}

func (m *MockContestsUC) IsParticipant(ctx context.Context, contestId, userId uuid.UUID) (bool, error) {
	args := m.Called(ctx, contestId, userId)
	return args.Bool(0), args.Error(1)
}

func (m *MockContestsUC) DeleteParticipant(ctx context.Context, contestId, userId uuid.UUID) error {
	args := m.Called(ctx, contestId, userId)
	return args.Error(0)
}

func (m *MockContestsUC) ListParticipants(ctx context.Context, filter models.ParticipantsFilter) (*models.UsersList, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.UsersList), args.Error(1)
}

func (m *MockContestsUC) GetMonitor(ctx context.Context, contestId uuid.UUID) (*models.Monitor, error) {
	args := m.Called(ctx, contestId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Monitor), args.Error(1)
}

type MockPermissionsClient struct {
	mock.Mock
}

func (m *MockPermissionsClient) CheckSolutionPermission(ctx context.Context, subjectID, solutionID, permission string) (bool, error) {
	args := m.Called(ctx, subjectID, solutionID, permission)
	return args.Bool(0), args.Error(1)
}

func (m *MockPermissionsClient) CheckContestPermission(ctx context.Context, subjectID, contestID, permission string) (bool, error) {
	args := m.Called(ctx, subjectID, contestID, permission)
	return args.Bool(0), args.Error(1)
}

func (m *MockPermissionsClient) CreateSolutionRelation(ctx context.Context, userID, solutionID, relation string) error {
	args := m.Called(ctx, userID, solutionID, relation)
	return args.Error(0)
}

func (m *MockPermissionsClient) CanViewContest(ctx context.Context, userID uuid.UUID, contest *models.Contest) (bool, error) {
	args := m.Called(ctx, userID, contest)
	return args.Bool(0), args.Error(1)
}

func (m *MockPermissionsClient) CanCreateSolution(ctx context.Context, userID uuid.UUID, contest *models.Contest) (bool, error) {
	args := m.Called(ctx, userID, contest)
	return args.Bool(0), args.Error(1)
}

type MockUsersUC struct {
	mock.Mock
}

func (m *MockUsersUC) CreateUser(ctx context.Context, user *models.UserCreation) (uuid.UUID, error) {
	args := m.Called(ctx, user)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockUsersUC) ReadUserById(ctx context.Context, id uuid.UUID) (*models.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUsersUC) ReadUserByKratosId(ctx context.Context, kratosId string) (*models.User, error) {
	args := m.Called(ctx, kratosId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUsersUC) SearchUsers(ctx context.Context, searchQuery, role string, page, pageSize int) ([]*models.User, int, error) {
	args := m.Called(ctx, searchQuery, role, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*models.User), args.Int(1), args.Error(2)
}

func (m *MockUsersUC) UpdateUserInIndex(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func setupFiberApp() *fiber.App {
	app := fiber.New()
	return app
}

func createMockSession(userID string) *ory.Session {
	active := true
	return &ory.Session{
		Active: &active,
		Identity: &ory.Identity{
			Id: userID,
		},
	}
}

func TestGetSolution_Success(t *testing.T) {
	app := setupFiberApp()
	mockSolutionsUC := new(MockSolutionsUC)
	mockContestsUC := new(MockContestsUC)
	mockPermissions := new(MockPermissionsClient)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockSolutionsUC, mockContestsUC, mockPermissions, mockUsersUC)

	solutionID := uuid.New()
	userID := uuid.New()
	kratosID := uuid.New().String()

	expectedUser := &models.User{
		Id:       userID,
		KratosId: &kratosID,
		Username: "testuser",
	}

	expectedSolution := &models.Submission{
		Id:       solutionID,
		UserId:   userID,
		Solution: "test solution",
		State:    models.Accepted,
	}

	mockUsersUC.On("ReadUserByKratosId", mock.Anything, kratosID).Return(expectedUser, nil)
	mockSolutionsUC.On("GetSubmissions", mock.Anything, solutionID).Return(expectedSolution, nil)

	app.Get("/submissions/:solution_id", func(c *fiber.Ctx) error {
		c.Locals("session", createMockSession(kratosID))
		return handlers.GetSolution(c, solutionID)
	})

	req := httptest.NewRequest("GET", "/submissions/"+solutionID.String(), nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response testerv1.GetSolutionResponseModel
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &response)

	assert.Equal(t, solutionID, response.Solution.Id)
	mockSolutionsUC.AssertExpectations(t)
	mockUsersUC.AssertExpectations(t)
}

func TestCreateSolution_Success(t *testing.T) {
	app := setupFiberApp()
	mockSolutionsUC := new(MockSolutionsUC)
	mockContestsUC := new(MockContestsUC)
	mockPermissions := new(MockPermissionsClient)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockSolutionsUC, mockContestsUC, mockPermissions, mockUsersUC)

	userID := uuid.New()
	kratosID := uuid.New().String()
	problemID := uuid.New()
	contestID := uuid.New()
	solutionID := uuid.New()

	expectedUser := &models.User{
		Id:       userID,
		KratosId: &kratosID,
		Username: "testuser",
	}

	expectedContest := &models.Contest{
		Id:    contestID,
		Title: "Test Contest",
	}

	mockUsersUC.On("ReadUserByKratosId", mock.Anything, kratosID).Return(expectedUser, nil)
	mockContestsUC.On("GetContest", mock.Anything, contestID).Return(expectedContest, nil)
	mockPermissions.On("CanCreateSolution", mock.Anything, userID, expectedContest).Return(true, nil)
	mockSolutionsUC.On("CreateSubmission", mock.Anything, mock.AnythingOfType("*models.SubmissionCreation")).Return(solutionID, nil)

	params := testerv1.CreateSolutionParams{
		ProblemId: problemID,
		ContestId: contestID,
		Language:  int32(models.Cpp),
	}

	app.Post("/submissions", func(c *fiber.Ctx) error {
		c.Locals("session", createMockSession(kratosID))
		return handlers.CreateSolution(c, params)
	})

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("solution", "solution.cpp")
	part.Write([]byte("#include <iostream>\nint main() { return 0; }"))
	writer.Close()

	req := httptest.NewRequest("POST", "/submissions", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	mockSolutionsUC.AssertExpectations(t)
	mockPermissions.AssertExpectations(t)
	mockUsersUC.AssertExpectations(t)
	mockContestsUC.AssertExpectations(t)
}
