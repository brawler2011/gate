//go:build integration

package postgres_test

import (
	"io"
	"strings"
	"testing"

	"github.com/brawler2011/gate/backend/internal/problems/builder"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func (h *pipelineHarness) upload(t *testing.T, change model.UploadChange, files map[string]string) {
	t.Helper()
	current, err := h.repo.GetDraft(h.ctx, h.draft.ProblemID)
	require.NoError(t, err)
	readers := map[string]io.Reader{}
	for slot, text := range files {
		readers[slot] = strings.NewReader(text)
	}
	_, err = h.uploads.Upload(h.ctx, h.draft.ProblemID, current.Revision, change, readers)
	require.NoError(t, err)
}

// Uploaded checker and validator tests run in every build, which fails naming the tests whose expectation misses.
func TestProblemPipelineRunsCheckerAndValidatorTests(t *testing.T) {
	t.Parallel()
	h := newPipelineHarness(t, 1<<20, 256<<20)
	generator, main, subtask := "gen", "main", uuid.New()
	h.save(t, func(d *model.Draft) {
		d.Checker = checker("python3", "import sys\nout, ans = (open(f).read().split() for f in sys.argv[2:4])\n"+
			"sys.exit(2 if not all(t.isdigit() for t in out) else 0 if out == ans else 1)\n")
		d.Validators = []model.DraftValidator{{DraftNamedProgram: named("v", "python3", "import sys\na, b = map(int, sys.stdin.read().split())\n"+
			"assert 0 <= a <= 100 and 0 <= b <= 100\nassert sys.argv[1:] != ['--group', 'small'] or max(a, b) < 10\n")}}
		d.Generators = []model.DraftNamedProgram{named(generator, "python3", "print(1, 2)\n")}
		d.Solutions = []model.DraftSolution{solutionProgram(main, "python3", "print(sum(map(int, input().split())))\n")}
		d.MainSolution = &main
		d.Subtasks = []model.DraftSubtask{{ID: subtask, Ordinal: 1, Policy: "complete"}}
		d.Tests = []model.DraftTest{{ID: uuid.New(), Ordinal: 1, SubtaskID: subtask, Input: model.DraftInput{Mode: "generator", Generator: &generator}, Answer: model.DraftAnswer{Mode: "generated", Solution: &main}}}
	})
	checkerTests := []struct{ output, verdict string }{{"3", "OK"}, {"4", "WA"}, {"three", "PE"}}
	for _, c := range checkerTests {
		h.upload(t, model.UploadChange{CheckerTest: &model.DraftCheckerTest{ID: uuid.New(), Verdict: c.verdict}}, map[string]string{"input": "1 2\n", "output": c.output + "\n", "answer": "3\n"})
	}
	validatorTests := []struct {
		input, group string
		valid        bool
	}{{"1 2", "", true}, {"200 1", "", false}, {"20 1", "small", false}, {"20 1", "", true}}
	for _, c := range validatorTests {
		h.upload(t, model.UploadChange{Validator: "v", ValidatorTest: &model.DraftValidatorTest{ID: uuid.New(), Valid: c.valid, Group: c.group}}, map[string]string{"input": c.input + "\n"})
	}

	build := h.enqueueAndClaim(t)
	_, err := h.pipeline().Execute(h.ctx, build)
	require.NoError(t, err)
	stored, err := h.repo.GetBuild(h.ctx, build.ID)
	require.NoError(t, err)
	verdicts := []string{}
	for _, r := range stored.ProgramTests {
		require.Empty(t, r.Mismatch)
		verdicts = append(verdicts, r.Verdict)
	}
	require.Equal(t, []string{"OK", "WA", "PE", "valid", "invalid", "invalid", "valid"}, verdicts)

	h.save(t, func(d *model.Draft) {
		d.Checker.Tests[1].Verdict = "OK"
		d.Validators[0].Tests[3].Valid = false
	})
	build = h.enqueueAndClaim(t)
	_, err = h.pipeline().Execute(h.ctx, build)
	var failure *builder.Failure
	require.ErrorAs(t, err, &failure)
	require.Equal(t, "program-tests", failure.Stage)
	require.Equal(t, "checker test 2: WA, expected OK\nvalidator v test 4: valid, expected invalid", failure.Message)
}
