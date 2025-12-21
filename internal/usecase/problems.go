package usecase

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gate149/core/internal/domain/interfaces"
	"github.com/gate149/core/internal/domain/models"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
)

type ProblemsUseCase struct {
	repo         interfaces.ProblemsRepo
	pandocClient interfaces.PandocClient
}

func NewProblemsUseCase(repo interfaces.ProblemsRepo, pandocClient interfaces.PandocClient) *ProblemsUseCase {
	return &ProblemsUseCase{
		repo:         repo,
		pandocClient: pandocClient,
	}
}

func (uc *ProblemsUseCase) CreateProblem(ctx context.Context, input *models.CreateProblemInput) (uuid.UUID, error) {
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

func (uc *ProblemsUseCase) GetProblemById(ctx context.Context, id uuid.UUID) (models.Problem, error) {
	return uc.repo.GetProblemById(ctx, id)
}

func (uc *ProblemsUseCase) UpdateProblem(ctx context.Context, id uuid.UUID, problem *models.ProblemUpdate) error {
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

	return nil
}

func (uc *ProblemsUseCase) DeleteProblem(ctx context.Context, id uuid.UUID) error {
	return uc.repo.DeleteProblem(ctx, id)
}

func (uc *ProblemsUseCase) ListProblems(ctx context.Context, filter *models.ProblemsFilter) (*models.ProblemsList, error) {
	problems, total, err := uc.repo.ListProblems(ctx, filter)
	if err != nil {
		return nil, err
	}

	domainProblems := make([]models.Problem, len(problems))
	for i, p := range problems {
		domainProblems[i] = models.Problem{
			ID:          p.ID,
			Title:       p.Title,
			MemoryLimit: p.MemoryLimit,
			TimeLimit:   p.TimeLimit,
			CreatedAt:   p.CreatedAt,
			UpdatedAt:   p.UpdatedAt,
		}
	}

	return &models.ProblemsList{
		Problems:   domainProblems,
		Pagination: models.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

func (uc *ProblemsUseCase) GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (models.ProblemMember, error) {
	return uc.repo.GetProblemMember(ctx, problemId, userId)
}

func (uc *ProblemsUseCase) CreateProblemTests(ctx context.Context, problemId uuid.UUID, tests []models.ProblemTest) error {
	modelTests := make(models.ProblemTests, len(tests))
	for i, t := range tests {
		modelTests[i] = models.ProblemTest{
			ID:        t.ID,
			ProblemID: t.ProblemID,
			Ordinal:   t.Ordinal,
			Input:     t.Input,
			Output:    t.Output,
		}
	}

	err := uc.repo.CreateProblemTests(ctx, modelTests)
	if err != nil {
		return err
	}

	return nil
}

func (uc *ProblemsUseCase) GetProblemTests(ctx context.Context, problemId uuid.UUID) ([]models.ProblemTest, error) {
	return uc.repo.GetProblemTests(ctx, problemId)
}

func (uc *ProblemsUseCase) DeleteProblemTests(ctx context.Context, problemId uuid.UUID) error {
	return uc.repo.DeleteProblemTests(ctx, problemId)
}

func (uc *ProblemsUseCase) UploadProblemTests(ctx context.Context, problemId uuid.UUID, zipData []byte) error {
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

	var tests []models.ProblemTest
	for num, input := range inputs {
		if output, ok := outputs[num]; ok {
			ordinal, _ := strconv.ParseInt(num, 10, 32)
			tests = append(tests, models.ProblemTest{
				ID:        uuid.New(),
				ProblemID: problemId,
				Ordinal:   int32(ordinal),
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
		tests[i].Ordinal = int32(i + 1)
	}

	// Delete existing tests
	if err := uc.DeleteProblemTests(ctx, problemId); err != nil {
		return err
	}

	// Create new tests
	return uc.CreateProblemTests(ctx, problemId, tests)
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
