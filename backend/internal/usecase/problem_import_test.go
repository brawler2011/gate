package usecase

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg/formats/gfmt"
	"github.com/gate149/gate/backend/pkg/storage"
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
	workspaceStorage := NewWorkspaceStorage(store, "test-workshop")
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

	manifestBytes, ok := problemsRepo.manifests[problemID]
	require.True(t, ok)

	var manifest models.ProblemManifest
	require.NoError(t, json.Unmarshal(manifestBytes, &manifest))
	assert.Equal(t, "APlusB", manifest.Statement.Title)
}

func TestImportProblemPackageICPCFallbackWithNestedRoot(t *testing.T) {
	ctx := context.Background()
	problemID := uuid.New()

	problemsRepo := newProblemImportMockProblemsRepo()
	problemsRepo.problems[problemID] = models.Problem{ID: problemID, Title: "Repository Problem Title"}

	store := storage.NewLocalStorage(t.TempDir())
	workspaceStorage := NewWorkspaceStorage(store, "test-workshop")
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

	manifestBytes, ok := problemsRepo.manifests[problemID]
	require.True(t, ok)

	var manifest models.ProblemManifest
	require.NoError(t, json.Unmarshal(manifestBytes, &manifest))
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
	manifests map[uuid.UUID][]byte
	problems  map[uuid.UUID]models.Problem
}

func newProblemImportMockProblemsRepo() *problemImportMockProblemsRepo {
	return &problemImportMockProblemsRepo{
		manifests: make(map[uuid.UUID][]byte),
		problems:  make(map[uuid.UUID]models.Problem),
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

func (m *problemImportMockProblemsRepo) ListProblems(_ context.Context, _ *models.ProblemsFilter) ([]models.Problem, int32, error) {
	return nil, 0, nil
}

func (m *problemImportMockProblemsRepo) UpdateProblem(_ context.Context, _ uuid.UUID, _ *models.ProblemUpdate) error {
	return nil
}

func (m *problemImportMockProblemsRepo) UpdateProblemLimits(_ context.Context, _ uuid.UUID, _, _ int) error {
	return nil
}

func (m *problemImportMockProblemsRepo) GetProblemManifest(_ context.Context, id uuid.UUID) ([]byte, error) {
	return m.manifests[id], nil
}

func (m *problemImportMockProblemsRepo) UpdateProblemManifest(_ context.Context, id uuid.UUID, manifest []byte) error {
	m.manifests[id] = append([]byte(nil), manifest...)
	return nil
}
