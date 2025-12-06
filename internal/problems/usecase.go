package problems

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gate149/core/internal/cache"
	"github.com/gate149/core/internal/domain"
	"github.com/gate149/core/internal/models"
	problemssqlc "github.com/gate149/core/internal/problems/sqlc"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
)

type Repo interface {
	CreateProblem(ctx context.Context, params *models.CreateProblemParams) error
	GetProblemById(ctx context.Context, id uuid.UUID) (problemssqlc.Problem, error)
	UpdateProblem(ctx context.Context, id uuid.UUID, problem *models.ProblemUpdate) error
	DeleteProblem(ctx context.Context, id uuid.UUID) error
	ListProblems(ctx context.Context, filter *models.ProblemsFilter) ([]problemssqlc.ListProblemsRow, int64, error)

	CreateProblemMember(ctx context.Context, params *models.CreateProblemMemberParams) error
	GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (problemssqlc.ProblemMember, error)

	CreateProblemTests(ctx context.Context, tests models.ProblemTests) error
	GetProblemTests(ctx context.Context, problemId uuid.UUID) ([]problemssqlc.GetProblemTestsRow, error)
	DeleteProblemTests(ctx context.Context, problemId uuid.UUID) error

	// Test Groups
	CreateTestGroup(ctx context.Context, id uuid.UUID, problemId uuid.UUID, ordinal int64, name string, points int64, isSample bool) error
	GetTestGroup(ctx context.Context, id uuid.UUID) (problemssqlc.TestGroup, error)
	GetTestGroupsByProblem(ctx context.Context, problemId uuid.UUID) ([]problemssqlc.TestGroup, error)
	UpdateTestGroup(ctx context.Context, id uuid.UUID, name *string, points *int64, isSample *bool) error
	DeleteTestGroup(ctx context.Context, id uuid.UUID) error

	// Problem Samples
	CreateProblemSample(ctx context.Context, id uuid.UUID, problemId uuid.UUID, ordinal int64, input string, output string) error
	GetProblemSamples(ctx context.Context, problemId uuid.UUID) ([]problemssqlc.ProblemSample, error)
	DeleteProblemSample(ctx context.Context, id uuid.UUID) error
}

type PandocClient interface {
	BatchConvertLatexToHtml5(ctx context.Context, texts []string) ([]string, error)
}

type UseCase struct {
	repo         Repo
	cache        cache.Cache
	pandocClient PandocClient
}

func NewUseCase(repo Repo, cache cache.Cache, pandocClient PandocClient) *UseCase {
	return &UseCase{
		repo:         repo,
		cache:        cache,
		pandocClient: pandocClient,
	}
}

func (uc *UseCase) CreateProblem(ctx context.Context, input *models.CreateProblemInput) (uuid.UUID, error) {
	id := uuid.New()

	params := &models.CreateProblemParams{
		Id:     id,
		Title:  input.Title,
		UserId: input.UserId,
	}

	if err := uc.repo.CreateProblem(ctx, params); err != nil {
		return uuid.Nil, err
	}

	// Add creator as owner
	memberParams := &models.CreateProblemMemberParams{
		ProblemId: id,
		UserId:    input.UserId,
		Role:      models.ProblemRoleOwner,
	}

	if err := uc.repo.CreateProblemMember(ctx, memberParams); err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (uc *UseCase) GetProblemById(ctx context.Context, id uuid.UUID) (domain.Problem, error) {
	// Cache-aside
	var cached domain.Problem
	if err := uc.cache.Get(ctx, cache.ProblemKey(id), &cached); err == nil {
		return cached, nil
	}

	problem, err := uc.repo.GetProblemById(ctx, id)
	if err != nil {
		return domain.Problem{}, err
	}

	domainProblem := domain.ProblemFromSqlc(problem)
	if err := uc.cache.Set(ctx, cache.ProblemKey(id), domainProblem, cache.ProblemTTL); err != nil {
		slog.Error("failed to cache problem", "error", err, "problem_id", id)
	}

	return domainProblem, nil
}

func (uc *UseCase) UpdateProblem(ctx context.Context, id uuid.UUID, problem *models.ProblemUpdate) error {
	// Process latex if needed
	if problem.Legend != nil || problem.InputFormat != nil || problem.OutputFormat != nil || problem.Notes != nil || problem.Scoring != nil {
		texts := make([]string, 5)
		if problem.Legend != nil {
			texts[0] = *problem.Legend
		}
		if problem.InputFormat != nil {
			texts[1] = *problem.InputFormat
		}
		if problem.OutputFormat != nil {
			texts[2] = *problem.OutputFormat
		}
		if problem.Notes != nil {
			texts[3] = *problem.Notes
		}
		if problem.Scoring != nil {
			texts[4] = *problem.Scoring
		}

		htmls, err := uc.pandocClient.BatchConvertLatexToHtml5(ctx, texts)
		if err != nil {
			return pkg.Wrap(err, nil, "failed to convert latex to html")
		}

		if problem.Legend != nil {
			problem.LegendHtml = &htmls[0]
		}
		if problem.InputFormat != nil {
			problem.InputFormatHtml = &htmls[1]
		}
		if problem.OutputFormat != nil {
			problem.OutputFormatHtml = &htmls[2]
		}
		if problem.Notes != nil {
			problem.NotesHtml = &htmls[3]
		}
		if problem.Scoring != nil {
			problem.ScoringHtml = &htmls[4]
		}
	}

	if err := uc.repo.UpdateProblem(ctx, id, problem); err != nil {
		return err
	}

	// Invalidate cache
	_ = uc.cache.Delete(ctx, cache.ProblemKey(id))

	// Invalidate all contest_problem caches that contain this problem
	// Pattern: contest_problem:*:{problemId}
	pattern := "contest_problem:*:" + id.String()
	_ = uc.cache.DeleteByPattern(ctx, pattern)

	return nil
}

func (uc *UseCase) DeleteProblem(ctx context.Context, id uuid.UUID) error {
	if err := uc.repo.DeleteProblem(ctx, id); err != nil {
		return err
	}

	// Invalidate cache
	_ = uc.cache.Delete(ctx, cache.ProblemKey(id))
	_ = uc.cache.Delete(ctx, cache.ProblemTestsKey(id)) // also invalidate tests

	// Invalidate all contest_problem caches that contain this problem
	// Pattern: contest_problem:*:{problemId}
	pattern := "contest_problem:*:" + id.String()
	_ = uc.cache.DeleteByPattern(ctx, pattern)

	return nil
}

func (uc *UseCase) ListProblems(ctx context.Context, filter *models.ProblemsFilter) (*domain.ProblemsList, error) {
	problems, total, err := uc.repo.ListProblems(ctx, filter)
	if err != nil {
		return nil, err
	}

	domainProblems := make([]domain.Problem, len(problems))
	for i, p := range problems {
		domainProblems[i] = domain.Problem{
			ID:          p.ID,
			Title:       p.Title,
			MemoryLimit: int64(p.MemoryLimit),
			TimeLimit:   int64(p.TimeLimit),
			CreatedAt:   p.CreatedAt,
			UpdatedAt:   p.UpdatedAt,
		}
	}

	return &domain.ProblemsList{
		Problems:   domainProblems,
		Pagination: domain.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

func (uc *UseCase) GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (domain.ProblemMember, error) {
	// Cache-aside
	key := cache.ProblemMemberKey(problemId, userId)
	var cached domain.ProblemMember
	if err := uc.cache.Get(ctx, key, &cached); err == nil {
		return cached, nil
	}

	member, err := uc.repo.GetProblemMember(ctx, problemId, userId)
	if err != nil {
		return domain.ProblemMember{}, err
	}

	// Manual mapping
	var pID uuid.UUID
	if member.ProblemID.Valid {
		pID = member.ProblemID.Bytes
	}

	domainMember := domain.ProblemMember{
		ProblemID: pID,
		UserID:    member.UserID,
		Role:      string(member.Role),
	}

	if err := uc.cache.Set(ctx, key, domainMember, cache.PermissionTTL); err != nil {
		slog.Error("failed to cache problem member", "error", err, "key", key)
	}

	return domainMember, nil
}

func (uc *UseCase) CreateProblemTests(ctx context.Context, problemId uuid.UUID, tests []domain.ProblemTest) error {
	modelTests := make(models.ProblemTests, len(tests))
	for i, t := range tests {
		modelTests[i] = models.ProblemTest{
			Id:        t.ID,
			ProblemId: t.ProblemID,
			Ordinal:   t.Ordinal,
			Input:     t.Input,
			Output:    t.Output,
		}
	}

	err := uc.repo.CreateProblemTests(ctx, modelTests)
	if err != nil {
		return err
	}

	// Invalidate cache
	_ = uc.cache.Delete(ctx, cache.ProblemTestsKey(problemId))

	return nil
}

func (uc *UseCase) GetProblemTests(ctx context.Context, problemId uuid.UUID) ([]domain.ProblemTest, error) {
	// Cache-aside
	key := cache.ProblemTestsKey(problemId)
	var cached []domain.ProblemTest
	if err := uc.cache.Get(ctx, key, &cached); err == nil {
		return cached, nil
	}

	tests, err := uc.repo.GetProblemTests(ctx, problemId)
	if err != nil {
		return nil, err
	}

	domainTests := make([]domain.ProblemTest, len(tests))
	for i, t := range tests {
		domainTests[i] = domain.ProblemTestFromGetRow(t)
	}

	if err := uc.cache.Set(ctx, key, domainTests, cache.ProblemTTL); err != nil {
		slog.Error("failed to cache problem tests", "error", err, "key", key)
	}

	return domainTests, nil
}

func (uc *UseCase) DeleteProblemTests(ctx context.Context, problemId uuid.UUID) error {
	err := uc.repo.DeleteProblemTests(ctx, problemId)
	if err != nil {
		return err
	}

	// Invalidate cache
	_ = uc.cache.Delete(ctx, cache.ProblemTestsKey(problemId))

	return nil
}

func (uc *UseCase) UploadProblemTests(ctx context.Context, problemId uuid.UUID, zipData []byte) error {
	// Parse zip
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "failed to open zip file")
	}

	inputs := make(map[string]string)
	outputs := make(map[string]string)

	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		name := file.Name
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)

		// Check if input or output
		// Assuming format: input/1.txt, output/1.txt or just 1.in, 1.out
		// Just simple heuristic: if name contains "input" or ends with .in -> input
		// if name contains "output" or ends with .out -> output

		// Read file content
		f, err := file.Open()
		if err != nil {
			return pkg.Wrap(pkg.ErrBadInput, err, "failed to read file in zip")
		}
		contentBytes, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return pkg.Wrap(pkg.ErrBadInput, err, "failed to read file content")
		}
		content := string(contentBytes)

		// Normalize newlines? Maybe later.

		if strings.HasSuffix(name, ".in") || strings.Contains(name, "input") {
			// Extract number
			numStr := extractNumber(base)
			if numStr != "" {
				inputs[numStr] = content
			}
		} else if strings.HasSuffix(name, ".out") || strings.HasSuffix(name, ".ans") || strings.Contains(name, "output") {
			numStr := extractNumber(base)
			if numStr != "" {
				outputs[numStr] = content
			}
		}
	}

	var tests []domain.ProblemTest
	for num, input := range inputs {
		if output, ok := outputs[num]; ok {
			ordinal, _ := strconv.ParseInt(num, 10, 64)
			tests = append(tests, domain.ProblemTest{
				ID:        uuid.New(),
				ProblemID: problemId,
				Ordinal:   ordinal,
				Input:     input,
				Output:    output,
			})
		}
	}

	if len(tests) == 0 {
		return pkg.Wrap(pkg.ErrBadInput, nil, "no valid tests found in zip (pairs of input/output)")
	}

	// Sort by ordinal
	sort.Slice(tests, func(i, j int) bool {
		return tests[i].Ordinal < tests[j].Ordinal
	})

	// Re-assign ordinals to be sequential 1..N
	for i := range tests {
		tests[i].Ordinal = int64(i + 1)
	}

	// Delete existing tests
	if err := uc.DeleteProblemTests(ctx, problemId); err != nil {
		return err
	}

	// Create new tests
	return uc.CreateProblemTests(ctx, problemId, tests)
}

// Test Groups

func (uc *UseCase) CreateTestGroup(ctx context.Context, problemId uuid.UUID, ordinal int64, name string, points int64, isSample bool) (uuid.UUID, error) {
	id := uuid.New()
	err := uc.repo.CreateTestGroup(ctx, id, problemId, ordinal, name, points, isSample)
	if err != nil {
		return uuid.Nil, pkg.Wrap(err, nil, "can't create test group")
	}
	return id, nil
}

func (uc *UseCase) GetTestGroup(ctx context.Context, id uuid.UUID) (domain.TestGroup, error) {
	group, err := uc.repo.GetTestGroup(ctx, id)
	if err != nil {
		return domain.TestGroup{}, pkg.Wrap(err, nil, "can't get test group")
	}

	return domain.TestGroup{
		ID:        group.ID,
		ProblemID: group.ProblemID,
		Ordinal:   int64(group.Ordinal),
		Name:      group.Name,
		Points:    int64(group.Points),
		IsSample:  group.IsSample,
		CreatedAt: group.CreatedAt,
	}, nil
}

func (uc *UseCase) GetTestGroupsByProblem(ctx context.Context, problemId uuid.UUID) ([]domain.TestGroup, error) {
	groups, err := uc.repo.GetTestGroupsByProblem(ctx, problemId)
	if err != nil {
		return nil, pkg.Wrap(err, nil, "can't get test groups")
	}

	domainGroups := make([]domain.TestGroup, len(groups))
	for i, g := range groups {
		domainGroups[i] = domain.TestGroup{
			ID:        g.ID,
			ProblemID: g.ProblemID,
			Ordinal:   int64(g.Ordinal),
			Name:      g.Name,
			Points:    int64(g.Points),
			IsSample:  g.IsSample,
			CreatedAt: g.CreatedAt,
		}
	}

	return domainGroups, nil
}

func (uc *UseCase) UpdateTestGroup(ctx context.Context, id uuid.UUID, name *string, points *int64, isSample *bool) error {
	err := uc.repo.UpdateTestGroup(ctx, id, name, points, isSample)
	if err != nil {
		return pkg.Wrap(err, nil, "can't update test group")
	}
	return nil
}

func (uc *UseCase) DeleteTestGroup(ctx context.Context, id uuid.UUID) error {
	err := uc.repo.DeleteTestGroup(ctx, id)
	if err != nil {
		return pkg.Wrap(err, nil, "can't delete test group")
	}
	return nil
}

// Problem Samples

func (uc *UseCase) CreateProblemSample(ctx context.Context, problemId uuid.UUID, ordinal int64, input string, output string) (uuid.UUID, error) {
	id := uuid.New()
	err := uc.repo.CreateProblemSample(ctx, id, problemId, ordinal, input, output)
	if err != nil {
		return uuid.Nil, pkg.Wrap(err, nil, "can't create problem sample")
	}
	return id, nil
}

func (uc *UseCase) GetProblemSamples(ctx context.Context, problemId uuid.UUID) ([]domain.ProblemSample, error) {
	samples, err := uc.repo.GetProblemSamples(ctx, problemId)
	if err != nil {
		return nil, pkg.Wrap(err, nil, "can't get problem samples")
	}

	domainSamples := make([]domain.ProblemSample, len(samples))
	for i, s := range samples {
		domainSamples[i] = domain.ProblemSample{
			ID:        s.ID,
			ProblemID: s.ProblemID,
			Ordinal:   int64(s.Ordinal),
			Input:     s.Input,
			Output:    s.Output,
			CreatedAt: s.CreatedAt,
		}
	}

	return domainSamples, nil
}

func (uc *UseCase) DeleteProblemSample(ctx context.Context, id uuid.UUID) error {
	err := uc.repo.DeleteProblemSample(ctx, id)
	if err != nil {
		return pkg.Wrap(err, nil, "can't delete problem sample")
	}
	return nil
}

func extractNumber(s string) string {
	// Extract numeric part from filename
	// E.g. "input/test1" -> "1"
	// "1" -> "1"
	var numStr string
	for _, r := range s {
		if r >= '0' && r <= '9' {
			numStr += string(r)
		}
	}
	return numStr
}
