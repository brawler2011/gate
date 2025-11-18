package contests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	testerv1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/internal/permissions"
	"github.com/gate149/core/pkg"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	ory "github.com/ory/client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock implementations

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

func (m *MockContestsUC) UpdateContest(ctx context.Context, id uuid.UUID, contestUpdate models.ContestUpdate) error {
	args := m.Called(ctx, id, contestUpdate)
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

type MockProblemsUC struct {
	mock.Mock
}

func (m *MockProblemsUC) CreateProblem(ctx context.Context, title string) (uuid.UUID, error) {
	args := m.Called(ctx, title)
	return args.Get(0).(uuid.UUID), args.Error(1)
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

func (m *MockProblemsUC) DeleteProblem(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockProblemsUC) ListProblems(ctx context.Context, filter models.ProblemsFilter) (*models.ProblemsList, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ProblemsList), args.Error(1)
}

func (m *MockProblemsUC) UpdateProblem(ctx context.Context, id uuid.UUID, problemUpdate *models.ProblemUpdate) error {
	args := m.Called(ctx, id, problemUpdate)
	return args.Error(0)
}

func (m *MockProblemsUC) UploadProblem(ctx context.Context, id uuid.UUID, r io.ReaderAt, size int64) error {
	args := m.Called(ctx, id, r, size)
	return args.Error(0)
}

type MockPermissionsUC struct {
	mock.Mock
}

func (m *MockPermissionsUC) CreatePermission(ctx context.Context, resourceType string, resourceID uuid.UUID, userID uuid.UUID, relation string) error {
	args := m.Called(ctx, resourceType, resourceID, userID, relation)
	return args.Error(0)
}

func (m *MockPermissionsUC) DeletePermission(ctx context.Context, resourceType string, resourceID uuid.UUID, userID uuid.UUID, relation string) error {
	args := m.Called(ctx, resourceType, resourceID, userID, relation)
	return args.Error(0)
}

func (m *MockPermissionsUC) CanViewContest(ctx context.Context, userID uuid.UUID, contest *models.Contest) (bool, error) {
	args := m.Called(ctx, userID, contest)
	return args.Bool(0), args.Error(1)
}

func (m *MockPermissionsUC) CanEditContest(ctx context.Context, userID uuid.UUID, contestID uuid.UUID) (bool, error) {
	args := m.Called(ctx, userID, contestID)
	return args.Bool(0), args.Error(1)
}

func (m *MockPermissionsUC) CanAdminContest(ctx context.Context, userID uuid.UUID, contestID uuid.UUID) (bool, error) {
	args := m.Called(ctx, userID, contestID)
	return args.Bool(0), args.Error(1)
}

func (m *MockPermissionsUC) CanViewMonitor(ctx context.Context, userID uuid.UUID, contest *models.Contest) (bool, error) {
	args := m.Called(ctx, userID, contest)
	return args.Bool(0), args.Error(1)
}

type MockUsersUC struct {
	mock.Mock
}

func (m *MockUsersUC) ReadUserByKratosId(ctx context.Context, kratosId string) (*models.User, error) {
	args := m.Called(ctx, kratosId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

// Helper functions

func setupFiberApp() *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			// Convert pkg.CustomError to appropriate HTTP status code
			statusCode := pkg.ToREST(err)
			return c.Status(statusCode).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})
	return app
}

func createMockSession(userID string) *ory.Session {
	active := true
	identity := &ory.Identity{
		Id: userID,
	}
	session := ory.Session{
		Active:   &active,
		Identity: identity,
	}
	return (*ory.Session)(&session)
}

func createTestUser(id uuid.UUID, kratosId string) *models.User {
	return &models.User{
		Id:        id,
		Username:  "testuser",
		Role:      "user",
		KratosId:  &kratosId,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func createTestContest(id uuid.UUID, isPrivate bool) *models.Contest {
	return &models.Contest{
		Id:             id,
		Title:          "Test Contest",
		IsPrivate:      isPrivate,
		MonitorEnabled: true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

// Tests

func TestCreateContest_Success(t *testing.T) {
	app := setupFiberApp()
	mockContestsUC := new(MockContestsUC)
	mockProblemsUC := new(MockProblemsUC)
	mockPermissionsUC := new(MockPermissionsUC)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockProblemsUC, mockContestsUC, mockPermissionsUC, mockUsersUC)

	userID := uuid.New()
	contestID := uuid.New()
	userIDStr := userID.String()
	kratosID := "kratos-" + userIDStr

	user := createTestUser(userID, kratosID)

	mockUsersUC.On("ReadUserByKratosId", mock.Anything, kratosID).Return(user, nil)
	mockContestsUC.On("CreateContest", mock.Anything, mock.MatchedBy(func(creation models.ContestCreation) bool {
		return creation.Title == "Test Contest"
	})).Return(contestID, nil)
	mockPermissionsUC.On("CreatePermission", mock.Anything, permissions.ResourceContest, contestID, userID, permissions.RelationOwner).Return(nil)

	params := testerv1.CreateContestParams{
		Title: "Test Contest",
	}

	app.Post("/contests", func(c *fiber.Ctx) error {
		c.Locals(sessionKey, createMockSession(kratosID))
		return handlers.CreateContest(c, params)
	})

	req := httptest.NewRequest("POST", "/contests", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response testerv1.CreationResponseModel
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &response)

	assert.Equal(t, contestID, response.Id)
	mockContestsUC.AssertExpectations(t)
	mockUsersUC.AssertExpectations(t)
}

func TestCreateContest_EmptyTitle(t *testing.T) {
	app := setupFiberApp()
	mockContestsUC := new(MockContestsUC)
	mockProblemsUC := new(MockProblemsUC)
	mockPermissionsUC := new(MockPermissionsUC)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockProblemsUC, mockContestsUC, mockPermissionsUC, mockUsersUC)

	userID := uuid.New()
	userIDStr := userID.String()
	kratosID := "kratos-" + userIDStr

	user := createTestUser(userID, kratosID)

	mockUsersUC.On("ReadUserByKratosId", mock.Anything, kratosID).Return(user, nil)

	params := testerv1.CreateContestParams{
		Title: "",
	}

	app.Post("/contests", func(c *fiber.Ctx) error {
		c.Locals(sessionKey, createMockSession(kratosID))
		return handlers.CreateContest(c, params)
	})

	req := httptest.NewRequest("POST", "/contests", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)

	mockUsersUC.AssertExpectations(t)
	mockContestsUC.AssertNotCalled(t, "CreateContest")
}

func TestCreateContest_ShortTitle(t *testing.T) {
	app := setupFiberApp()
	mockContestsUC := new(MockContestsUC)
	mockProblemsUC := new(MockProblemsUC)
	mockPermissionsUC := new(MockPermissionsUC)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockProblemsUC, mockContestsUC, mockPermissionsUC, mockUsersUC)

	userID := uuid.New()
	userIDStr := userID.String()
	kratosID := "kratos-" + userIDStr

	user := createTestUser(userID, kratosID)

	mockUsersUC.On("ReadUserByKratosId", mock.Anything, kratosID).Return(user, nil)

	params := testerv1.CreateContestParams{
		Title: "AB",
	}

	app.Post("/contests", func(c *fiber.Ctx) error {
		c.Locals(sessionKey, createMockSession(kratosID))
		return handlers.CreateContest(c, params)
	})

	req := httptest.NewRequest("POST", "/contests", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)

	mockUsersUC.AssertExpectations(t)
	mockContestsUC.AssertNotCalled(t, "CreateContest")
}

func TestGetContest_Success(t *testing.T) {
	app := setupFiberApp()
	mockContestsUC := new(MockContestsUC)
	mockProblemsUC := new(MockProblemsUC)
	mockPermissionsUC := new(MockPermissionsUC)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockProblemsUC, mockContestsUC, mockPermissionsUC, mockUsersUC)

	userID := uuid.New()
	contestID := uuid.New()
	userIDStr := userID.String()
	kratosID := "kratos-" + userIDStr

	user := createTestUser(userID, kratosID)
	contest := createTestContest(contestID, false)
	problems := []*models.ContestProblemsListItem{}

	mockUsersUC.On("ReadUserByKratosId", mock.Anything, kratosID).Return(user, nil)
	mockContestsUC.On("GetContest", mock.Anything, contestID).Return(contest, nil)
	mockPermissionsUC.On("ContestPermissionGet", mock.Anything, userID, contest).Return(true, nil)
	mockContestsUC.On("GetContestProblems", mock.Anything, contestID).Return(problems, nil)

	app.Get("/contests/:id", func(c *fiber.Ctx) error {
		c.Locals(sessionKey, createMockSession(kratosID))
		return handlers.GetContest(c, contestID)
	})

	req := httptest.NewRequest("GET", "/contests/"+contestID.String(), nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response testerv1.GetContestResponseModel
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &response)

	assert.Equal(t, contestID, response.Contest.Id)
	assert.Equal(t, "Test Contest", response.Contest.Title)
	mockContestsUC.AssertExpectations(t)
	mockUsersUC.AssertExpectations(t)
	mockPermissionsUC.AssertExpectations(t)
}

func TestGetContest_NoPermission(t *testing.T) {
	app := setupFiberApp()
	mockContestsUC := new(MockContestsUC)
	mockProblemsUC := new(MockProblemsUC)
	mockPermissionsUC := new(MockPermissionsUC)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockProblemsUC, mockContestsUC, mockPermissionsUC, mockUsersUC)

	userID := uuid.New()
	contestID := uuid.New()
	userIDStr := userID.String()
	kratosID := "kratos-" + userIDStr

	user := createTestUser(userID, kratosID)
	contest := createTestContest(contestID, true)

	mockUsersUC.On("ReadUserByKratosId", mock.Anything, kratosID).Return(user, nil)
	mockContestsUC.On("GetContest", mock.Anything, contestID).Return(contest, nil)
	mockPermissionsUC.On("ContestPermissionGet", mock.Anything, userID, contest).Return(false, nil)

	app.Get("/contests/:id", func(c *fiber.Ctx) error {
		c.Locals(sessionKey, createMockSession(kratosID))
		return handlers.GetContest(c, contestID)
	})

	req := httptest.NewRequest("GET", "/contests/"+contestID.String(), nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 403, resp.StatusCode)

	mockContestsUC.AssertExpectations(t)
	mockUsersUC.AssertExpectations(t)
	mockPermissionsUC.AssertExpectations(t)
	mockContestsUC.AssertNotCalled(t, "GetContestProblems")
}

func TestUpdateContest_Success(t *testing.T) {
	app := setupFiberApp()
	mockContestsUC := new(MockContestsUC)
	mockProblemsUC := new(MockProblemsUC)
	mockPermissionsUC := new(MockPermissionsUC)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockProblemsUC, mockContestsUC, mockPermissionsUC, mockUsersUC)

	userID := uuid.New()
	contestID := uuid.New()
	userIDStr := userID.String()
	kratosID := "kratos-" + userIDStr

	user := createTestUser(userID, kratosID)
	newTitle := "Updated Contest"
	isPrivate := true
	monitorEnabled := false

	mockUsersUC.On("ReadUserByKratosId", mock.Anything, kratosID).Return(user, nil)
	mockPermissionsUC.On("EditContest", mock.Anything, userID, contestID).Return(true, nil)
	mockContestsUC.On("UpdateContest", mock.Anything, contestID, mock.MatchedBy(func(update models.ContestUpdate) bool {
		return *update.Title == newTitle && *update.IsPrivate == isPrivate && *update.MonitorEnabled == monitorEnabled
	})).Return(nil)

	reqBody := testerv1.UpdateContestRequestModel{
		Title:          &newTitle,
		IsPrivate:      &isPrivate,
		MonitorEnabled: &monitorEnabled,
	}

	bodyBytes, _ := json.Marshal(reqBody)

	app.Put("/contests/:id", func(c *fiber.Ctx) error {
		c.Locals(sessionKey, createMockSession(kratosID))
		return handlers.UpdateContest(c, contestID)
	})

	req := httptest.NewRequest("PUT", "/contests/"+contestID.String(), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	mockContestsUC.AssertExpectations(t)
	mockUsersUC.AssertExpectations(t)
	mockPermissionsUC.AssertExpectations(t)
}

func TestUpdateContest_NoPermission(t *testing.T) {
	app := setupFiberApp()
	mockContestsUC := new(MockContestsUC)
	mockProblemsUC := new(MockProblemsUC)
	mockPermissionsUC := new(MockPermissionsUC)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockProblemsUC, mockContestsUC, mockPermissionsUC, mockUsersUC)

	userID := uuid.New()
	contestID := uuid.New()
	userIDStr := userID.String()
	kratosID := "kratos-" + userIDStr

	user := createTestUser(userID, kratosID)
	newTitle := "Updated Contest"

	mockUsersUC.On("ReadUserByKratosId", mock.Anything, kratosID).Return(user, nil)
	mockPermissionsUC.On("EditContest", mock.Anything, userID, contestID).Return(false, nil)

	reqBody := testerv1.UpdateContestRequestModel{
		Title: &newTitle,
	}

	bodyBytes, _ := json.Marshal(reqBody)

	app.Put("/contests/:id", func(c *fiber.Ctx) error {
		c.Locals(sessionKey, createMockSession(kratosID))
		return handlers.UpdateContest(c, contestID)
	})

	req := httptest.NewRequest("PUT", "/contests/"+contestID.String(), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 403, resp.StatusCode)

	mockUsersUC.AssertExpectations(t)
	mockPermissionsUC.AssertExpectations(t)
	mockContestsUC.AssertNotCalled(t, "UpdateContest")
}

func TestDeleteContest_Success(t *testing.T) {
	app := setupFiberApp()
	mockContestsUC := new(MockContestsUC)
	mockProblemsUC := new(MockProblemsUC)
	mockPermissionsUC := new(MockPermissionsUC)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockProblemsUC, mockContestsUC, mockPermissionsUC, mockUsersUC)

	userID := uuid.New()
	contestID := uuid.New()
	userIDStr := userID.String()
	kratosID := "kratos-" + userIDStr

	user := createTestUser(userID, kratosID)

	mockUsersUC.On("ReadUserByKratosId", mock.Anything, kratosID).Return(user, nil)
	mockPermissionsUC.On("AdminContest", mock.Anything, userID, contestID).Return(true, nil)
	mockContestsUC.On("DeleteContest", mock.Anything, contestID).Return(nil)

	app.Delete("/contests/:id", func(c *fiber.Ctx) error {
		c.Locals(sessionKey, createMockSession(kratosID))
		return handlers.DeleteContest(c, contestID)
	})

	req := httptest.NewRequest("DELETE", "/contests/"+contestID.String(), nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	mockContestsUC.AssertExpectations(t)
	mockUsersUC.AssertExpectations(t)
	mockPermissionsUC.AssertExpectations(t)
}

func TestListContests_Success(t *testing.T) {
	app := setupFiberApp()
	mockContestsUC := new(MockContestsUC)
	mockProblemsUC := new(MockProblemsUC)
	mockPermissionsUC := new(MockPermissionsUC)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockProblemsUC, mockContestsUC, mockPermissionsUC, mockUsersUC)

	userID := uuid.New()
	userIDStr := userID.String()
	kratosID := "kratos-" + userIDStr

	user := createTestUser(userID, kratosID)
	contestID := uuid.New()
	contest := createTestContest(contestID, false)

	contestsList := &models.ContestsList{
		Contests: []*models.Contest{contest},
		Pagination: models.Pagination{
			Page:  1,
			Total: 1,
		},
	}

	mockUsersUC.On("ReadUserByKratosId", mock.Anything, kratosID).Return(user, nil)
	mockContestsUC.On("ListContests", mock.Anything, mock.MatchedBy(func(filter models.ContestsFilter) bool {
		return filter.Page == int64(1) && filter.PageSize == int64(10)
	})).Return(contestsList, nil)

	params := testerv1.ListContestsParams{
		Page:     1,
		PageSize: 10,
	}

	app.Get("/contests", func(c *fiber.Ctx) error {
		c.Locals(sessionKey, createMockSession(kratosID))
		return handlers.ListContests(c, params)
	})

	req := httptest.NewRequest("GET", "/contests?page=1&page_size=10", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response testerv1.ListContestsResponseModel
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &response)

	assert.Equal(t, 1, len(response.Contests))
	assert.Equal(t, contestID, response.Contests[0].Id)
	mockContestsUC.AssertExpectations(t)
	mockUsersUC.AssertExpectations(t)
}

func TestListContests_WithTitleFilter(t *testing.T) {
	app := setupFiberApp()
	mockContestsUC := new(MockContestsUC)
	mockProblemsUC := new(MockProblemsUC)
	mockPermissionsUC := new(MockPermissionsUC)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockProblemsUC, mockContestsUC, mockPermissionsUC, mockUsersUC)

	userID := uuid.New()
	userIDStr := userID.String()
	kratosID := "kratos-" + userIDStr

	user := createTestUser(userID, kratosID)
	titleFilter := "Test"

	contestsList := &models.ContestsList{
		Contests:   []*models.Contest{},
		Pagination: models.Pagination{Page: 1, Total: 0},
	}

	mockUsersUC.On("ReadUserByKratosId", mock.Anything, kratosID).Return(user, nil)
	mockContestsUC.On("ListContests", mock.Anything, mock.MatchedBy(func(filter models.ContestsFilter) bool {
		return filter.Page == int64(1) && filter.PageSize == int64(10) && filter.Search != nil && *filter.Search == titleFilter
	})).Return(contestsList, nil)

	params := testerv1.ListContestsParams{
		Page:     1,
		PageSize: 10,
		Title:    &titleFilter,
	}

	app.Get("/contests", func(c *fiber.Ctx) error {
		c.Locals(sessionKey, createMockSession(kratosID))
		return handlers.ListContests(c, params)
	})

	req := httptest.NewRequest("GET", "/contests?page=1&page_size=10&title=Test", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	mockContestsUC.AssertExpectations(t)
	mockUsersUC.AssertExpectations(t)
}

func TestCreateContestProblem_Success(t *testing.T) {
	app := setupFiberApp()
	mockContestsUC := new(MockContestsUC)
	mockProblemsUC := new(MockProblemsUC)
	mockPermissionsUC := new(MockPermissionsUC)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockProblemsUC, mockContestsUC, mockPermissionsUC, mockUsersUC)

	userID := uuid.New()
	contestID := uuid.New()
	problemID := uuid.New()
	userIDStr := userID.String()
	kratosID := "kratos-" + userIDStr

	user := createTestUser(userID, kratosID)

	mockUsersUC.On("ReadUserByKratosId", mock.Anything, kratosID).Return(user, nil)
	mockPermissionsUC.On("EditContest", mock.Anything, userID, contestID).Return(true, nil)
	mockContestsUC.On("ContestProblemCreation", mock.Anything, contestID, problemID).Return(nil)

	params := testerv1.CreateContestProblemParams{
		ProblemId: problemID,
	}

	app.Post("/contests/:contest_id/problems", func(c *fiber.Ctx) error {
		c.Locals(sessionKey, createMockSession(kratosID))
		return handlers.CreateContestProblem(c, contestID, params)
	})

	req := httptest.NewRequest("POST", "/contests/"+contestID.String()+"/problems", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	mockContestsUC.AssertExpectations(t)
	mockUsersUC.AssertExpectations(t)
	mockPermissionsUC.AssertExpectations(t)
}

func TestGetContestProblem_Success(t *testing.T) {
	app := setupFiberApp()
	mockContestsUC := new(MockContestsUC)
	mockProblemsUC := new(MockProblemsUC)
	mockPermissionsUC := new(MockPermissionsUC)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockProblemsUC, mockContestsUC, mockPermissionsUC, mockUsersUC)

	userID := uuid.New()
	contestID := uuid.New()
	problemID := uuid.New()
	userIDStr := userID.String()
	kratosID := "kratos-" + userIDStr

	user := createTestUser(userID, kratosID)
	contest := createTestContest(contestID, false)
	contestProblem := &models.ContestProblem{
		ProblemId:   problemID,
		Title:       "Test Problem",
		TimeLimit:   1000,
		MemoryLimit: 256,
		Position:    1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	mockUsersUC.On("ReadUserByKratosId", mock.Anything, kratosID).Return(user, nil)
	mockContestsUC.On("GetContest", mock.Anything, contestID).Return(contest, nil)
	mockPermissionsUC.On("ContestPermissionGet", mock.Anything, userID, contest).Return(true, nil)
	mockContestsUC.On("GetContestProblem", mock.Anything, contestID, problemID).Return(contestProblem, nil)

	app.Get("/contests/:contest_id/problems/:problem_id", func(c *fiber.Ctx) error {
		c.Locals(sessionKey, createMockSession(kratosID))
		return handlers.GetContestProblem(c, contestID, problemID)
	})

	req := httptest.NewRequest("GET", "/contests/"+contestID.String()+"/problems/"+problemID.String(), nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response testerv1.GetContestProblemResponseModel
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &response)

	assert.Equal(t, problemID, response.Problem.ProblemId)
	assert.Equal(t, "Test Problem", response.Problem.Title)
	mockContestsUC.AssertExpectations(t)
	mockUsersUC.AssertExpectations(t)
	mockPermissionsUC.AssertExpectations(t)
}

func TestDeleteContestProblem_Success(t *testing.T) {
	app := setupFiberApp()
	mockContestsUC := new(MockContestsUC)
	mockProblemsUC := new(MockProblemsUC)
	mockPermissionsUC := new(MockPermissionsUC)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockProblemsUC, mockContestsUC, mockPermissionsUC, mockUsersUC)

	userID := uuid.New()
	contestID := uuid.New()
	problemID := uuid.New()
	userIDStr := userID.String()
	kratosID := "kratos-" + userIDStr

	user := createTestUser(userID, kratosID)

	mockUsersUC.On("ReadUserByKratosId", mock.Anything, kratosID).Return(user, nil)
	mockPermissionsUC.On("EditContest", mock.Anything, userID, contestID).Return(true, nil)
	mockContestsUC.On("DeleteContestProblem", mock.Anything, contestID, problemID).Return(nil)

	app.Delete("/contests/:contest_id/problems/:problem_id", func(c *fiber.Ctx) error {
		c.Locals(sessionKey, createMockSession(kratosID))
		return handlers.DeleteContestProblem(c, contestID, problemID)
	})

	req := httptest.NewRequest("DELETE", "/contests/"+contestID.String()+"/problems/"+problemID.String(), nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	mockContestsUC.AssertExpectations(t)
	mockUsersUC.AssertExpectations(t)
	mockPermissionsUC.AssertExpectations(t)
}

func TestCreateParticipant_Success(t *testing.T) {
	app := setupFiberApp()
	mockContestsUC := new(MockContestsUC)
	mockProblemsUC := new(MockProblemsUC)
	mockPermissionsUC := new(MockPermissionsUC)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockProblemsUC, mockContestsUC, mockPermissionsUC, mockUsersUC)

	userID := uuid.New()
	contestID := uuid.New()
	participantID := uuid.New()
	userIDStr := userID.String()
	kratosID := "kratos-" + userIDStr

	user := createTestUser(userID, kratosID)

	mockUsersUC.On("ReadUserByKratosId", mock.Anything, kratosID).Return(user, nil)
	mockPermissionsUC.On("EditContest", mock.Anything, userID, contestID).Return(true, nil)
	mockPermissionsUC.On("CreatePermission", mock.Anything, permissions.ResourceContest, contestID, participantID, permissions.RelationParticipant).Return(nil)

	params := testerv1.CreateParticipantParams{
		UserId: participantID,
	}

	app.Post("/contests/:contest_id/participants", func(c *fiber.Ctx) error {
		c.Locals(sessionKey, createMockSession(kratosID))
		return handlers.CreateParticipant(c, contestID, params)
	})

	req := httptest.NewRequest("POST", "/contests/"+contestID.String()+"/participants", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	mockUsersUC.AssertExpectations(t)
	mockPermissionsUC.AssertExpectations(t)
}

func TestDeleteParticipant_Success(t *testing.T) {
	app := setupFiberApp()
	mockContestsUC := new(MockContestsUC)
	mockProblemsUC := new(MockProblemsUC)
	mockPermissionsUC := new(MockPermissionsUC)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockProblemsUC, mockContestsUC, mockPermissionsUC, mockUsersUC)

	userID := uuid.New()
	contestID := uuid.New()
	participantID := uuid.New()
	userIDStr := userID.String()
	kratosID := "kratos-" + userIDStr

	user := createTestUser(userID, kratosID)

	mockUsersUC.On("ReadUserByKratosId", mock.Anything, kratosID).Return(user, nil)
	mockPermissionsUC.On("EditContest", mock.Anything, userID, contestID).Return(true, nil)
	mockPermissionsUC.On("DeletePermission", mock.Anything, permissions.ResourceContest, contestID, participantID, permissions.RelationParticipant).Return(nil)

	params := testerv1.DeleteParticipantParams{
		UserId: participantID,
	}

	app.Delete("/contests/:contest_id/participants", func(c *fiber.Ctx) error {
		c.Locals(sessionKey, createMockSession(kratosID))
		return handlers.DeleteParticipant(c, contestID, params)
	})

	req := httptest.NewRequest("DELETE", "/contests/"+contestID.String()+"/participants", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	mockUsersUC.AssertExpectations(t)
	mockPermissionsUC.AssertExpectations(t)
}

func TestListParticipants_Success(t *testing.T) {
	app := setupFiberApp()
	mockContestsUC := new(MockContestsUC)
	mockProblemsUC := new(MockProblemsUC)
	mockPermissionsUC := new(MockPermissionsUC)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockProblemsUC, mockContestsUC, mockPermissionsUC, mockUsersUC)

	userID := uuid.New()
	contestID := uuid.New()
	participantID := uuid.New()
	userIDStr := userID.String()
	kratosID := "kratos-" + userIDStr

	user := createTestUser(userID, kratosID)
	contest := createTestContest(contestID, false)
	participant := createTestUser(participantID, "kratos-participant")

	participantsList := &models.UsersList{
		Users: []*models.User{participant},
		Pagination: models.Pagination{
			Page:  1,
			Total: 1,
		},
	}

	mockUsersUC.On("ReadUserByKratosId", mock.Anything, kratosID).Return(user, nil)
	mockContestsUC.On("GetContest", mock.Anything, contestID).Return(contest, nil)
	mockPermissionsUC.On("ContestPermissionGet", mock.Anything, userID, contest).Return(true, nil)
	mockContestsUC.On("ListParticipants", mock.Anything, mock.MatchedBy(func(filter models.ParticipantsFilter) bool {
		return filter.ContestId == contestID && filter.Page == 1 && filter.PageSize == 10
	})).Return(participantsList, nil)

	params := testerv1.ListParticipantsParams{
		Page:     1,
		PageSize: 10,
	}

	app.Get("/contests/:contest_id/participants", func(c *fiber.Ctx) error {
		c.Locals(sessionKey, createMockSession(kratosID))
		return handlers.ListParticipants(c, contestID, params)
	})

	req := httptest.NewRequest("GET", "/contests/"+contestID.String()+"/participants?page=1&page_size=10", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response testerv1.ListUsersResponseModel
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &response)

	assert.Equal(t, 1, len(response.Users))
	assert.Equal(t, participantID, response.Users[0].Id)
	mockContestsUC.AssertExpectations(t)
	mockUsersUC.AssertExpectations(t)
	mockPermissionsUC.AssertExpectations(t)
}

func TestGetMonitor_Success(t *testing.T) {
	app := setupFiberApp()
	mockContestsUC := new(MockContestsUC)
	mockProblemsUC := new(MockProblemsUC)
	mockPermissionsUC := new(MockPermissionsUC)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockProblemsUC, mockContestsUC, mockPermissionsUC, mockUsersUC)

	userID := uuid.New()
	contestID := uuid.New()
	problemID := uuid.New()
	userIDStr := userID.String()
	kratosID := "kratos-" + userIDStr

	user := createTestUser(userID, kratosID)
	contest := createTestContest(contestID, false)

	monitor := &models.Monitor{
		Participants: []*models.ParticipantsStat{
			{
				UserId:   userID,
				Username: "testuser",
				Solved:   1,
				Penalty:  100,
				Attempts: []*models.ProblemAttempts{
					{
						ProblemId: problemID,
						Position:  1,
						FAttempts: 0,
						State:     nil,
					},
				},
			},
		},
		Summary: []*models.ProblemStatSummary{
			{
				ProblemId:   problemID,
				Position:    1,
				SAttempts:   1,
				UnsAttempts: 0,
				TAttempts:   1,
			},
		},
	}

	mockUsersUC.On("ReadUserByKratosId", mock.Anything, kratosID).Return(user, nil)
	mockContestsUC.On("GetContest", mock.Anything, contestID).Return(contest, nil)
	mockPermissionsUC.On("ViewMonitor", mock.Anything, userID, contest).Return(true, nil)
	mockContestsUC.On("GetMonitor", mock.Anything, contestID).Return(monitor, nil)

	app.Get("/contests/:contest_id/monitor", func(c *fiber.Ctx) error {
		c.Locals(sessionKey, createMockSession(kratosID))
		return handlers.GetMonitor(c, contestID)
	})

	req := httptest.NewRequest("GET", "/contests/"+contestID.String()+"/monitor", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response testerv1.GetMonitorResponseModel
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &response)

	assert.Equal(t, 1, len(response.Participants))
	assert.Equal(t, 1, len(response.Summary))
	assert.Equal(t, "testuser", response.Participants[0].Username)
	assert.Equal(t, problemID, response.Summary[0].ProblemId)
	mockContestsUC.AssertExpectations(t)
	mockUsersUC.AssertExpectations(t)
	mockPermissionsUC.AssertExpectations(t)
}

func TestGetMonitor_NoPermission(t *testing.T) {
	app := setupFiberApp()
	mockContestsUC := new(MockContestsUC)
	mockProblemsUC := new(MockProblemsUC)
	mockPermissionsUC := new(MockPermissionsUC)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockProblemsUC, mockContestsUC, mockPermissionsUC, mockUsersUC)

	userID := uuid.New()
	contestID := uuid.New()
	userIDStr := userID.String()
	kratosID := "kratos-" + userIDStr

	user := createTestUser(userID, kratosID)
	contest := createTestContest(contestID, true)

	mockUsersUC.On("ReadUserByKratosId", mock.Anything, kratosID).Return(user, nil)
	mockContestsUC.On("GetContest", mock.Anything, contestID).Return(contest, nil)
	mockPermissionsUC.On("ViewMonitor", mock.Anything, userID, contest).Return(false, nil)

	app.Get("/contests/:contest_id/monitor", func(c *fiber.Ctx) error {
		c.Locals(sessionKey, createMockSession(kratosID))
		return handlers.GetMonitor(c, contestID)
	})

	req := httptest.NewRequest("GET", "/contests/"+contestID.String()+"/monitor", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 403, resp.StatusCode)

	mockContestsUC.AssertExpectations(t)
	mockUsersUC.AssertExpectations(t)
	mockPermissionsUC.AssertExpectations(t)
	mockContestsUC.AssertNotCalled(t, "GetMonitor")
}

func TestCreateContest_Unauthenticated(t *testing.T) {
	app := setupFiberApp()
	mockContestsUC := new(MockContestsUC)
	mockProblemsUC := new(MockProblemsUC)
	mockPermissionsUC := new(MockPermissionsUC)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockProblemsUC, mockContestsUC, mockPermissionsUC, mockUsersUC)

	params := testerv1.CreateContestParams{
		Title: "Test Contest",
	}

	app.Post("/contests", func(c *fiber.Ctx) error {
		// No session in context
		return handlers.CreateContest(c, params)
	})

	req := httptest.NewRequest("POST", "/contests", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)

	mockContestsUC.AssertNotCalled(t, "CreateContest")
	mockUsersUC.AssertNotCalled(t, "ReadUserByKratosId")
}

func TestListContests_InvalidPage(t *testing.T) {
	app := setupFiberApp()
	mockContestsUC := new(MockContestsUC)
	mockProblemsUC := new(MockProblemsUC)
	mockPermissionsUC := new(MockPermissionsUC)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockProblemsUC, mockContestsUC, mockPermissionsUC, mockUsersUC)

	userID := uuid.New()
	userIDStr := userID.String()
	kratosID := "kratos-" + userIDStr

	params := testerv1.ListContestsParams{
		Page:     0, // Invalid page
		PageSize: 10,
	}

	app.Get("/contests", func(c *fiber.Ctx) error {
		c.Locals(sessionKey, createMockSession(kratosID))
		return handlers.ListContests(c, params)
	})

	req := httptest.NewRequest("GET", "/contests?page=0&page_size=10", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)

	// Validation happens before user lookup, so ReadUserByKratosId should not be called
	mockUsersUC.AssertNotCalled(t, "ReadUserByKratosId")
	mockContestsUC.AssertNotCalled(t, "ListContests")
}

func TestListContests_InvalidPageSize(t *testing.T) {
	app := setupFiberApp()
	mockContestsUC := new(MockContestsUC)
	mockProblemsUC := new(MockProblemsUC)
	mockPermissionsUC := new(MockPermissionsUC)
	mockUsersUC := new(MockUsersUC)

	handlers := NewHandlers(mockProblemsUC, mockContestsUC, mockPermissionsUC, mockUsersUC)

	userID := uuid.New()
	userIDStr := userID.String()
	kratosID := "kratos-" + userIDStr

	params := testerv1.ListContestsParams{
		Page:     1,
		PageSize: 101, // Invalid page size (max is 100)
	}

	app.Get("/contests", func(c *fiber.Ctx) error {
		c.Locals(sessionKey, createMockSession(kratosID))
		return handlers.ListContests(c, params)
	})

	req := httptest.NewRequest("GET", "/contests?page=1&page_size=101", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)

	// Validation happens before user lookup, so ReadUserByKratosId should not be called
	mockUsersUC.AssertNotCalled(t, "ReadUserByKratosId")
	mockContestsUC.AssertNotCalled(t, "ListContests")
}
