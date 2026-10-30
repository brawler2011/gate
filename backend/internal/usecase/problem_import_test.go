package usecase

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/brawler2011/gate/backend/pkg"
	"sync"
	"testing"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestImportProblemPackagePolygonFallback(t *testing.T) {
	ctx := context.Background()
	problemID := uuid.New()

	problemsRepo := newProblemImportMockProblemsRepo()
	problemsRepo.problems[problemID] = models.Problem{ID: problemID, Title: "Repo Title"}

	store := storage.NewLocalStorage(t.TempDir())
	workspaceStorage := NewWorkspaceStorage(store, "test-workshop", problemsRepo)
	uc := NewProblemImportUseCase(problemsRepo, workspaceStorage)

	zipBytes := createZipBytes(t, map[string]string{
		"problem.xml": `<?xml version="1.0" encoding="UTF-8"?>
<problem short-name="APlusB" revision="1">
  <judging>
    <testset name="tests">
      <time-limit>1000</time-limit>
      <memory-limit>268435456</memory-limit>
      <test-count>1</test-count>
      <input-path-pattern>data/secret/%02d.in</input-path-pattern>
      <answer-path-pattern>data/secret/%02d.ans</answer-path-pattern>
      <tests>
        <test method="manual" sample="true" />
      </tests>
    </testset>
  </judging>
  <files>
    <executables></executables>
  </files>
  <statements>
    <statement language="en" charset="utf-8" type="application/x-tex" path="statements/en.html" />
  </statements>
</problem>`,
		"statements/en.html": "<h1>A+B</h1>",
		"data/secret/01.in":  "1 2\n",
		"data/secret/01.ans": "3\n",
	})

	res, err := uc.ImportProblemPackage(ctx, bytes.NewReader(zipBytes), int64(len(zipBytes)), problemID)
	require.NoError(t, err)
	require.NotNil(t, res)

	in, err := workspaceStorage.ReadFile(ctx, problemID, "tests/01.in")
	require.NoError(t, err)
	assert.Equal(t, "1 2\n", string(in))

	out, err := workspaceStorage.ReadFile(ctx, problemID, "tests/01.out")
	require.NoError(t, err)
	assert.Equal(t, "3\n", string(out))

	manifest, err := NewWorkshopUseCase(workspaceStorage, nil).GetManifest(ctx, problemID)
	require.NoError(t, err)
	assert.Equal(t, "APlusB", manifest.Statement.Title)
}

func TestImportProblemPackageICPCFallbackWithNestedRoot(t *testing.T) {
	ctx := context.Background()
	problemID := uuid.New()

	problemsRepo := newProblemImportMockProblemsRepo()
	problemsRepo.problems[problemID] = models.Problem{ID: problemID, Title: "Repository Problem Title"}

	store := storage.NewLocalStorage(t.TempDir())
	workspaceStorage := NewWorkspaceStorage(store, "test-workshop", problemsRepo)
	uc := NewProblemImportUseCase(problemsRepo, workspaceStorage)

	zipBytes := createZipBytes(t, map[string]string{
		"icpc-problem/problem.yaml": `name: "A + B"
limits:
  time: 1.5
  memory: 128
validation:
  type: "default"
`,
		"icpc-problem/statement/en/problem.md": "# A + B\nCalculate sum.",
		"icpc-problem/data/sample/sample1.in":  "1 2\n",
		"icpc-problem/data/sample/sample1.ans": "3\n",
		"icpc-problem/data/secret/test1.in":    "10 20\n",
		"icpc-problem/data/secret/test1.ans":   "30\n",
	})

	res, err := uc.ImportProblemPackage(ctx, bytes.NewReader(zipBytes), int64(len(zipBytes)), problemID)
	require.NoError(t, err)
	require.NotNil(t, res)

	in1, err := workspaceStorage.ReadFile(ctx, problemID, "tests/01.in")
	require.NoError(t, err)
	assert.Equal(t, "1 2\n", string(in1))

	out1, err := workspaceStorage.ReadFile(ctx, problemID, "tests/01.out")
	require.NoError(t, err)
	assert.Equal(t, "3\n", string(out1))

	in2, err := workspaceStorage.ReadFile(ctx, problemID, "tests/02.in")
	require.NoError(t, err)
	assert.Equal(t, "10 20\n", string(in2))

	out2, err := workspaceStorage.ReadFile(ctx, problemID, "tests/02.out")
	require.NoError(t, err)
	assert.Equal(t, "30\n", string(out2))

	yamlBytes, err := workspaceStorage.ReadFile(ctx, problemID, "problem.yaml")
	require.NoError(t, err)

	var prob gfmt.Problem
	require.NoError(t, yaml.Unmarshal(yamlBytes, &prob))
	testsMeta := GfmtProblemToTestsMetadata(&prob)

	require.Len(t, testsMeta.Tests, 2)
	assert.Equal(t, 1, testsMeta.Tests[0].Ordinal)
	assert.Equal(t, 2, testsMeta.Tests[1].Ordinal)

	manifest, err := NewWorkshopUseCase(workspaceStorage, nil).GetManifest(ctx, problemID)
	require.NoError(t, err)
	assert.Equal(t, "A + B", manifest.Statement.Title)
}

func createZipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for path, content := range files {
		w, err := zw.Create(path)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())

	return buf.Bytes()
}

type problemImportMockProblemsRepo struct {
	workspaceMu     sync.Mutex
	workspaceHashes map[uuid.UUID]string
	problems        map[uuid.UUID]models.Problem
}

func newProblemImportMockProblemsRepo() *problemImportMockProblemsRepo {
	return &problemImportMockProblemsRepo{
		problems: make(map[uuid.UUID]models.Problem),
	}
}

func (m *problemImportMockProblemsRepo) CreateProblem(_ context.Context, _ *models.CreateProblemParams) error {
	return nil
}

func (m *problemImportMockProblemsRepo) CreateProblemMember(_ context.Context, _ *models.CreateProblemMemberParams) error {
	return nil
}

func (m *problemImportMockProblemsRepo) CreateProblemTests(_ context.Context, _ models.ProblemTests) error {
	return nil
}

func (m *problemImportMockProblemsRepo) DeleteProblem(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *problemImportMockProblemsRepo) DeleteProblemTests(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *problemImportMockProblemsRepo) GetProblemById(_ context.Context, id uuid.UUID) (models.Problem, error) {
	m.workspaceMu.Lock()
	defer m.workspaceMu.Unlock()
	if problem, ok := m.problems[id]; ok {
		return problem, nil
	}
	return models.Problem{}, nil
}

func (m *problemImportMockProblemsRepo) GetProblemMember(_ context.Context, _, _ uuid.UUID) (models.ProblemMember, error) {
	return models.ProblemMember{}, nil
}

func (m *problemImportMockProblemsRepo) GetProblemTests(_ context.Context, _ uuid.UUID) ([]models.ProblemTest, error) {
	return nil, nil
}

func (m *problemImportMockProblemsRepo) GetProblemTeams(_ context.Context, _ uuid.UUID) ([]models.ProblemTeam, error) {
	return nil, nil
}

func (m *problemImportMockProblemsRepo) AddProblemTeam(_ context.Context, _, _ uuid.UUID, _ models.ProblemPermission) error {
	return nil
}

func (m *problemImportMockProblemsRepo) UpdateProblemTeamPermission(_ context.Context, _, _ uuid.UUID, _ models.ProblemPermission) error {
	return nil
}

func (m *problemImportMockProblemsRepo) RemoveProblemTeam(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

func (m *problemImportMockProblemsRepo) ListProblemMembers(_ context.Context, _ uuid.UUID) ([]models.ProblemMember, error) {
	return nil, nil
}

func (m *problemImportMockProblemsRepo) UpdateProblemMemberRole(_ context.Context, _, _ uuid.UUID, _ models.ProblemRole) error {
	return nil
}

func (m *problemImportMockProblemsRepo) RemoveProblemMember(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

func (m *problemImportMockProblemsRepo) ListProblems(_ context.Context, _ *models.ProblemsFilter) ([]models.Problem, int32, error) {
	return nil, 0, nil
}

func (m *problemImportMockProblemsRepo) ListDashboardProblems(_ context.Context, _ uuid.UUID, _ int32) ([]models.DashboardProblem, error) {
	return nil, nil
}

func (m *problemImportMockProblemsRepo) UpdateProblem(_ context.Context, _ uuid.UUID, _ *models.ProblemUpdate) error {
	return nil
}

func (m *problemImportMockProblemsRepo) GetWorkspaceHash(ctx context.Context, id uuid.UUID) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	m.workspaceMu.Lock()
	defer m.workspaceMu.Unlock()
	return m.workspaceHashes[id], nil
}
func (m *problemImportMockProblemsRepo) CommitWorkspace(ctx context.Context, id uuid.UUID, previousHash, hash string, metadata *models.WorkspaceMetadata) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.workspaceMu.Lock()
	defer m.workspaceMu.Unlock()
	if m.workspaceHashes[id] != previousHash {
		return pkg.ErrConflict
	}
	if m.workspaceHashes == nil {
		m.workspaceHashes = make(map[uuid.UUID]string)
	}
	m.workspaceHashes[id] = hash
	if metadata != nil {
		problem := m.problems[id]
		problem.Title = metadata.Title
		problem.TimeLimitMs = metadata.TimeLimitMs
		problem.MemoryLimitMb = metadata.MemoryLimitMb
		m.problems[id] = problem
	}
	return nil
}

// Seed the stored archive directly so tests can also represent corrupt existing
// configurations that the normal writer refuses to create.
func seedWorkspaceFiles(t *testing.T, ws *WorkspaceStorage, id uuid.UUID, files map[string]string) {
	t.Helper()
	data := createZipBytes(t, files)
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	previous, err := ws.repo.GetWorkspaceHash(context.Background(), id)
	require.NoError(t, err)
	require.NoError(t, ws.storage.UploadFile(context.Background(), ws.bucket, ws.getWorkspaceKey(id, hash), bytes.NewReader(data), "application/zip"))
	require.NoError(t, ws.repo.CommitWorkspace(context.Background(), id, previous, hash, nil))
}

func workshopFixtureFiles(manifest *models.ProblemManifest, files map[string]string) map[string]string {
	config := ManifestAndTestsToGfmtProblem(manifest, defaultTestsMetadata())
	config.Sources = make(map[string]gfmt.Source)
	for _, meta := range manifest.FilesMetadata {
		var deps []string
		for _, dep := range meta.Dependencies {
			deps = append(deps, dep.Filename)
		}
		config.Sources[meta.Filename] = gfmt.Source{Compiler: meta.Compiler, Dependencies: deps}
	}
	result := map[string]string{"tests/01.in": "invalid input\n"}
	for path, content := range files {
		result[path] = content
	}
	if content, ok := files["problem.yaml"]; ok {
		config.Subtasks = nil
		if err := yaml.Unmarshal([]byte(content), config); err != nil {
			return result
		}
	}
	data, err := yaml.Marshal(config)
	if err != nil {
		panic(err)
	}
	result["problem.yaml"] = string(data)
	return result
}
