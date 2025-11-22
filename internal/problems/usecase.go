package problems

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"
)

type Repo interface {
	CreateProblem(ctx context.Context, params *models.CreateProblemParams) error
	GetProblemById(ctx context.Context, id uuid.UUID) (*models.Problem, error)
	DeleteProblem(ctx context.Context, id uuid.UUID) error
	ListProblems(ctx context.Context, filter *models.ProblemsFilter) (*models.ProblemsList, error)
	UpdateProblem(ctx context.Context, id uuid.UUID, heading *models.ProblemUpdate) error
	GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (*models.ProblemMember, error)
	CreateProblemMember(ctx context.Context, params *models.CreateProblemMemberParams) error
	DeleteProblemTests(ctx context.Context, problemId uuid.UUID) error
	CreateProblemTests(ctx context.Context, tests models.ProblemTests) error
}

type Pandoc interface {
	ConvertLatexToHtml5(ctx context.Context, text string) (string, error)
	BatchConvertLatexToHtml5(ctx context.Context, texts []string) ([]string, error)
}

type UseCase struct {
	problemRepo  Repo
	pandocClient Pandoc
}

func NewUseCase(
	problemRepo Repo,
	pandocClient Pandoc,
) (*UseCase, error) {
	return &UseCase{
		problemRepo:  problemRepo,
		pandocClient: pandocClient,
	}, nil
}

func (u *UseCase) CreateProblem(ctx context.Context, input *models.CreateProblemInput) (uuid.UUID, error) {
	params := &models.CreateProblemParams{
		Id:     uuid.New(),
		Title:  input.Title,
		UserId: input.UserId,
	}

	// FIXME: use transaction here

	err := u.problemRepo.CreateProblem(ctx, params)
	if err != nil {
		return uuid.Nil, err
	}

	err = u.problemRepo.CreateProblemMember(ctx, &models.CreateProblemMemberParams{
		ProblemId: params.Id,
		UserId:    input.UserId,
		Role:      models.ProblemRoleOwner,
	})
	if err != nil {
		return uuid.Nil, err
	}

	return params.Id, nil
}

func (u *UseCase) GetProblemById(ctx context.Context, id uuid.UUID) (*models.Problem, error) {
	return u.problemRepo.GetProblemById(ctx, id)
}

func (u *UseCase) DeleteProblem(ctx context.Context, id uuid.UUID) error {
	return u.problemRepo.DeleteProblem(ctx, id)
}

func (u *UseCase) ListProblems(ctx context.Context, filter *models.ProblemsFilter) (*models.ProblemsList, error) {
	return u.problemRepo.ListProblems(ctx, filter)
}

func (u *UseCase) UpdateProblem(ctx context.Context, id uuid.UUID, problemUpdate *models.ProblemUpdate) error {
	if isEmpty(*problemUpdate) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "empty problem update")
	}

	problem, err := u.problemRepo.GetProblemById(ctx, id)
	if err != nil {
		return err
	}

	statement := models.ProblemStatement{
		Legend:       problem.Legend,
		InputFormat:  problem.InputFormat,
		OutputFormat: problem.OutputFormat,
		Notes:        problem.Notes,
		Scoring:      problem.Scoring,
	}

	if problemUpdate.Legend != nil {
		statement.Legend = *problemUpdate.Legend
	}
	if problemUpdate.InputFormat != nil {
		statement.InputFormat = *problemUpdate.InputFormat
	}
	if problemUpdate.OutputFormat != nil {
		statement.OutputFormat = *problemUpdate.OutputFormat
	}
	if problemUpdate.Notes != nil {
		statement.Notes = *problemUpdate.Notes
	}
	if problemUpdate.Scoring != nil {
		statement.Scoring = *problemUpdate.Scoring
	}

	builtStatement, err := build(ctx, u.pandocClient, trimSpaces(statement))
	if err != nil {
		return err
	}

	problemUpdate.LegendHtml = &builtStatement.LegendHtml
	problemUpdate.InputFormatHtml = &builtStatement.InputFormatHtml
	problemUpdate.OutputFormatHtml = &builtStatement.OutputFormatHtml
	problemUpdate.NotesHtml = &builtStatement.NotesHtml
	problemUpdate.ScoringHtml = &builtStatement.ScoringHtml

	err = u.problemRepo.UpdateProblem(ctx, id, problemUpdate)
	if err != nil {
		return errors.Join(err)
	}

	return nil
}

type ProblemProperties struct {
	Title string `json:"name"`

	TimeLimit   int64 `json:"timeLimit"`
	MemoryLimit int64 `json:"memoryLimit"`

	Legend       *string `json:"legend"`
	Scoring      *string `json:"scoring"`
	Notes        *string `json:"notes"`
	OutputFormat *string `json:"output"`
	InputFormat  *string `json:"input"`
}

func isEmpty(p models.ProblemUpdate) bool {
	return p.Title == nil &&
		p.Legend == nil &&
		p.InputFormat == nil &&
		p.OutputFormat == nil &&
		p.Notes == nil &&
		p.Scoring == nil &&
		p.MemoryLimit == nil &&
		p.TimeLimit == nil
}

func wrap(s string) string {
	return fmt.Sprintf("\\begin{document}\n%s\n\\end{document}\n", s)
}

func trimSpaces(statement models.ProblemStatement) models.ProblemStatement {
	return models.ProblemStatement{
		Legend:       strings.TrimSpace(statement.Legend),
		InputFormat:  strings.TrimSpace(statement.InputFormat),
		OutputFormat: strings.TrimSpace(statement.OutputFormat),
		Notes:        strings.TrimSpace(statement.Notes),
		Scoring:      strings.TrimSpace(statement.Scoring),
	}
}

func sanitize(statement models.Html5ProblemStatement) models.Html5ProblemStatement {
	p := bluemonday.UGCPolicy()

	p.AllowAttrs("class").Globally()
	p.AllowAttrs("style").Globally()
	p.AllowStyles("text-align").MatchingEnum("center", "left", "right").Globally()
	p.AllowStyles("display").MatchingEnum("block", "inline", "inline-block").Globally()

	p.AllowStandardURLs()
	p.AllowAttrs("cite").OnElements("blockquote", "q")
	p.AllowAttrs("href").OnElements("a", "area")
	p.AllowAttrs("src").OnElements("img")

	if statement.LegendHtml != "" {
		statement.LegendHtml = p.Sanitize(statement.LegendHtml)
	}
	if statement.InputFormatHtml != "" {
		statement.InputFormatHtml = p.Sanitize(statement.InputFormatHtml)
	}
	if statement.OutputFormatHtml != "" {
		statement.OutputFormatHtml = p.Sanitize(statement.OutputFormatHtml)
	}
	if statement.NotesHtml != "" {
		statement.NotesHtml = p.Sanitize(statement.NotesHtml)
	}
	if statement.ScoringHtml != "" {
		statement.ScoringHtml = p.Sanitize(statement.ScoringHtml)
	}

	return statement
}

func build(ctx context.Context, pandocClient Pandoc, p models.ProblemStatement) (models.Html5ProblemStatement, error) {
	p = trimSpaces(p)

	latex := models.ProblemStatement{}

	if p.Legend != "" {
		latex.Legend = wrap(p.Legend)
	}
	if p.InputFormat != "" {
		latex.InputFormat = wrap(p.InputFormat)
	}
	if p.OutputFormat != "" {
		latex.OutputFormat = wrap(p.OutputFormat)
	}
	if p.Notes != "" {
		latex.Notes = wrap(p.Notes)
	}
	if p.Scoring != "" {
		latex.Scoring = wrap(p.Scoring)
	}

	req := []string{
		latex.Legend,
		latex.InputFormat,
		latex.OutputFormat,
		latex.Notes,
		latex.Scoring,
	}

	res, err := pandocClient.BatchConvertLatexToHtml5(ctx, req)
	if err != nil {
		return models.Html5ProblemStatement{}, err
	}

	if len(res) != len(req) {
		return models.Html5ProblemStatement{}, fmt.Errorf("wrong number of fieilds returned: %d", len(res))
	}

	sanitizedStatement := sanitize(models.Html5ProblemStatement{
		LegendHtml:       res[0],
		InputFormatHtml:  res[1],
		OutputFormatHtml: res[2],
		NotesHtml:        res[3],
		ScoringHtml:      res[4],
	})

	return sanitizedStatement, nil
}

func (u *UseCase) GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (*models.ProblemMember, error) {
	return u.problemRepo.GetProblemMember(ctx, problemId, userId)
}

const (
	maxArchiveSize = 10 * 1024 * 1024 // 10 MB
	maxTestSize    = 1 * 1024 * 1024  // 1 MB
)

func (u *UseCase) UploadProblemTests(ctx context.Context, problemId uuid.UUID, zipData []byte) error {
	// Validate archive size
	if len(zipData) > maxArchiveSize {
		return pkg.Wrap(pkg.ErrBadInput, nil, "archive size exceeds 10 MB limit")
	}

	// Parse ZIP archive
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "failed to parse ZIP archive")
	}

	// Extract test files from ZIP
	testFiles := make(map[int]struct {
		input  *string
		output *string
	})

	for _, file := range zipReader.File {
		// Skip directories
		if file.FileInfo().IsDir() {
			continue
		}

		// Get filename without path
		fileName := filepath.Base(file.Name)

		// Parse filename (N.in or N.out)
		parts := strings.Split(fileName, ".")
		if len(parts) != 2 {
			continue
		}

		testNum, err := strconv.Atoi(parts[0])
		if err != nil || testNum <= 0 {
			continue
		}

		extension := parts[1]
		if extension != "in" && extension != "out" {
			continue
		}

		// Validate file size
		if file.UncompressedSize64 > maxTestSize {
			return pkg.Wrap(pkg.ErrBadInput, nil, fmt.Sprintf("test file %s exceeds 1 MB limit", fileName))
		}

		// Read file content
		rc, err := file.Open()
		if err != nil {
			return pkg.Wrap(pkg.ErrBadInput, err, fmt.Sprintf("failed to open file %s", fileName))
		}

		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return pkg.Wrap(pkg.ErrBadInput, err, fmt.Sprintf("failed to read file %s", fileName))
		}

		contentStr := string(content)

		// Store in map
		if testFiles[testNum].input == nil && testFiles[testNum].output == nil {
			testFiles[testNum] = struct {
				input  *string
				output *string
			}{}
		}

		test := testFiles[testNum]
		if extension == "in" {
			test.input = &contentStr
		} else {
			test.output = &contentStr
		}
		testFiles[testNum] = test
	}

	// Validate that all tests have both .in and .out files
	if len(testFiles) == 0 {
		return pkg.Wrap(pkg.ErrBadInput, nil, "no valid test files found in archive")
	}

	// Extract test numbers and sort them
	testNums := make([]int, 0, len(testFiles))
	for num := range testFiles {
		testNums = append(testNums, num)
	}
	sort.Ints(testNums)

	// Validate and build tests array
	tests := make(models.ProblemTests, 0, len(testFiles))
	for i, num := range testNums {
		test := testFiles[num]
		if test.input == nil {
			return pkg.Wrap(pkg.ErrBadInput, nil, fmt.Sprintf("missing %d.in file", num))
		}
		if test.output == nil {
			return pkg.Wrap(pkg.ErrBadInput, nil, fmt.Sprintf("missing %d.out file", num))
		}

		tests = append(tests, models.ProblemTest{
			ProblemId: problemId,
			Ordinal:   int64(i + 1), // Use sequential ordinals starting from 1
			Input:     *test.input,
			Output:    *test.output,
		})
	}

	// Delete old tests and insert new ones (in transaction via repository)
	if err := u.problemRepo.DeleteProblemTests(ctx, problemId); err != nil {
		return err
	}

	if err := u.problemRepo.CreateProblemTests(ctx, tests); err != nil {
		return err
	}

	return nil
}
