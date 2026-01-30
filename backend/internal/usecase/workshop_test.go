package usecase

import (
	"context"
	"testing"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg/problemformat"
	"github.com/gate149/gate/backend/pkg/vcs"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockVCSService is a mock implementation of vcs.Service
type MockVCSService struct {
	mock.Mock
}

func (m *MockVCSService) InitProblemRepo(ctx context.Context, problemID uuid.UUID) error {
	args := m.Called(ctx, problemID)
	return args.Error(0)
}

func (m *MockVCSService) DeleteProblemRepo(ctx context.Context, problemID uuid.UUID) error {
	args := m.Called(ctx, problemID)
	return args.Error(0)
}

func (m *MockVCSService) RepoExists(ctx context.Context, problemID uuid.UUID) bool {
	args := m.Called(ctx, problemID)
	return args.Bool(0)
}

func (m *MockVCSService) ReadFile(ctx context.Context, problemID uuid.UUID, path string) ([]byte, error) {
	args := m.Called(ctx, problemID, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockVCSService) WriteFile(ctx context.Context, problemID uuid.UUID, path string, content []byte) error {
	args := m.Called(ctx, problemID, path, content)
	return args.Error(0)
}

func (m *MockVCSService) DeleteFile(ctx context.Context, problemID uuid.UUID, path string) error {
	args := m.Called(ctx, problemID, path)
	return args.Error(0)
}

func (m *MockVCSService) ListFiles(ctx context.Context, problemID uuid.UUID, dirPath string) ([]vcs.FileEntry, error) {
	args := m.Called(ctx, problemID, dirPath)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]vcs.FileEntry), args.Error(1)
}

func (m *MockVCSService) Commit(ctx context.Context, problemID uuid.UUID, message, authorName, authorEmail string) (string, error) {
	args := m.Called(ctx, problemID, message, authorName, authorEmail)
	return args.String(0), args.Error(1)
}

func (m *MockVCSService) GetStatus(ctx context.Context, problemID uuid.UUID) ([]vcs.FileStatus, error) {
	args := m.Called(ctx, problemID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]vcs.FileStatus), args.Error(1)
}

func (m *MockVCSService) GetHistory(ctx context.Context, problemID uuid.UUID, limit int) ([]vcs.Commit, error) {
	args := m.Called(ctx, problemID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]vcs.Commit), args.Error(1)
}

func (m *MockVCSService) GetCommitDiff(ctx context.Context, problemID uuid.UUID, commitSHA string) ([]vcs.FileDiff, error) {
	args := m.Called(ctx, problemID, commitSHA)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]vcs.FileDiff), args.Error(1)
}

func (m *MockVCSService) GetCurrentSHA(ctx context.Context, problemID uuid.UUID) (string, error) {
	args := m.Called(ctx, problemID)
	return args.String(0), args.Error(1)
}

func (m *MockVCSService) HasUncommittedChanges(ctx context.Context, problemID uuid.UUID) (bool, error) {
	args := m.Called(ctx, problemID)
	return args.Bool(0), args.Error(1)
}

func (m *MockVCSService) LoadManifest(ctx context.Context, problemID uuid.UUID) (*problemformat.ProblemManifest, error) {
	args := m.Called(ctx, problemID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*problemformat.ProblemManifest), args.Error(1)
}

func (m *MockVCSService) SaveManifest(ctx context.Context, problemID uuid.UUID, manifest *problemformat.ProblemManifest) error {
	args := m.Called(ctx, problemID, manifest)
	return args.Error(0)
}

func (m *MockVCSService) ValidateRepoStructure(ctx context.Context, problemID uuid.UUID) error {
	args := m.Called(ctx, problemID)
	return args.Error(0)
}

func (m *MockVCSService) GetRepoPath(problemID uuid.UUID) string {
	args := m.Called(problemID)
	return args.String(0)
}

func (m *MockVCSService) InitDefaultManifest(ctx context.Context, problemID uuid.UUID, title string) error {
	args := m.Called(ctx, problemID, title)
	return args.Error(0)
}

func (m *MockVCSService) InitDefaultTestsMetadata(ctx context.Context, problemID uuid.UUID) error {
	args := m.Called(ctx, problemID)
	return args.Error(0)
}

func TestWorkshopUseCase_UpdateProblemFile(t *testing.T) {
	mockVCS := new(MockVCSService)
	ctx := context.Background()
	problemID := uuid.New()

	// Setup mock
	mockVCS.On("WriteFile", ctx, problemID, "test.txt", []byte("content")).Return(nil)

	// Create use case with nil for other dependencies (not needed for this test)
	uc := &WorkshopUseCase{
		vcsService: mockVCS,
	}

	// Test
	req := models.UpdateFileRequest{
		ProblemID: problemID,
		UserID:    uuid.Nil,
		Path:      "test.txt",
		Content:   []byte("content"),
	}

	err := uc.UpdateProblemFile(ctx, req)
	assert.NoError(t, err)

	mockVCS.AssertExpectations(t)
}

func TestWorkshopUseCase_ReadProblemFile(t *testing.T) {
	mockVCS := new(MockVCSService)
	ctx := context.Background()
	problemID := uuid.New()
	expectedContent := []byte("test content")

	// Setup mock
	mockVCS.On("ReadFile", ctx, problemID, "test.txt").Return(expectedContent, nil)

	uc := &WorkshopUseCase{
		vcsService: mockVCS,
	}

	// Test
	content, err := uc.ReadProblemFile(ctx, problemID, "test.txt")
	assert.NoError(t, err)
	assert.Equal(t, expectedContent, content)

	mockVCS.AssertExpectations(t)
}

func TestWorkshopUseCase_ListProblemFiles(t *testing.T) {
	mockVCS := new(MockVCSService)
	ctx := context.Background()
	problemID := uuid.New()
	expectedFiles := []vcs.FileEntry{
		{Path: "file1.txt", IsDirectory: false, Size: 100},
		{Path: "dir1", IsDirectory: true, Size: 0},
	}

	// Setup mock
	mockVCS.On("ListFiles", ctx, problemID, "").Return(expectedFiles, nil)

	uc := &WorkshopUseCase{
		vcsService: mockVCS,
	}

	// Test
	files, err := uc.ListProblemFiles(ctx, problemID, "")
	assert.NoError(t, err)
	assert.Equal(t, expectedFiles, files)

	mockVCS.AssertExpectations(t)
}

func TestWorkshopUseCase_GetWorkshopStatus(t *testing.T) {
	mockVCS := new(MockVCSService)
	ctx := context.Background()
	problemID := uuid.New()

	expectedStatuses := []vcs.FileStatus{
		{Path: "file1.txt", Staging: "modified", Worktree: "modified"},
	}
	expectedSHA := "abc123"

	// Setup mocks
	mockVCS.On("GetStatus", ctx, problemID).Return(expectedStatuses, nil)
	mockVCS.On("GetCurrentSHA", ctx, problemID).Return(expectedSHA, nil)
	mockVCS.On("HasUncommittedChanges", ctx, problemID).Return(true, nil)

	uc := &WorkshopUseCase{
		vcsService: mockVCS,
	}

	// Test
	status, err := uc.GetWorkshopStatus(ctx, problemID)
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, expectedSHA, status.CurrentSHA)
	assert.True(t, status.HasUncommittedChanges)
	assert.Equal(t, expectedStatuses, status.ModifiedFiles)

	mockVCS.AssertExpectations(t)
}
