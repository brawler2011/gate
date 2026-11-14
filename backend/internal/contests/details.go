package contests

import (
	"context"
	"io"
	"log/slog"
	"strings"

	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/booklet"
	"github.com/google/uuid"
)

type ContestDetails struct {
	Owner    *identitymodel.UserProfile
	Contest  contestsmodel.Contest
	Problems []contestsmodel.ContestProblem
}
type ContestProblemDetails struct {
	Link contestsmodel.ContestProblem
	problemsmodel.ProblemDetails
	// Assets maps statement asset paths to the content hashes that address them.
	Assets map[string]string
}
type ContestDetailsStore interface {
	GetContestByOrgLoginAndContestLogin(context.Context, string, string) (contestsmodel.Contest, error)
	GetContestProblem(context.Context, contestsmodel.ContestProblemGet) (contestsmodel.ContestProblem, error)
	GetContestProblems(context.Context, uuid.UUID) ([]contestsmodel.ContestProblem, error)
}
type StatementSource interface {
	ReadStatement(context.Context, uuid.UUID, uuid.UUID, string) problemsmodel.StatementContent
	OpenImage(context.Context, uuid.UUID, string) (io.ReadCloser, string, error)
}
type BookletCompiler interface {
	CompilePDF(context.Context, string) ([]byte, error)
}
type ContestOwners interface {
	GetUserProfileByID(context.Context, uuid.UUID) (identitymodel.UserProfile, error)
}
type DetailsReader struct {
	users       ContestOwners
	contests    ContestDetailsStore
	statements  StatementSource
	permissions Permissions
	compiler    BookletCompiler
}

func NewDetailsReader(contests ContestDetailsStore, statements StatementSource, permissions Permissions, compiler BookletCompiler, users ContestOwners) *DetailsReader {
	return &DetailsReader{users: users, contests: contests, statements: statements, permissions: permissions, compiler: compiler}
}
func (r *DetailsReader) authorizedContest(ctx context.Context, org, login string, action accessmodel.ContestAction) (contestsmodel.Contest, error) {
	contest, err := r.contests.GetContestByOrgLoginAndContestLogin(ctx, org, login)
	if err != nil {
		return contest, err
	}
	allowed, err := r.permissions.HasContestPermission(ctx, contest.ID, accessmodel.UserFromContext(ctx).Id, action)
	if err != nil {
		return contest, err
	}
	if !allowed {
		return contest, pkg.NoPermission
	}
	return contest, nil
}
func (r *DetailsReader) GetContestDetails(ctx context.Context, org, login string) (*ContestDetails, error) {
	contest, err := r.authorizedContest(ctx, org, login, accessmodel.ActionGetContest)
	if err != nil {
		return nil, err
	}
	allowed, err := r.permissions.HasContestPermission(ctx, contest.ID, accessmodel.UserFromContext(ctx).Id, accessmodel.ActionGetContestProblem)
	if err != nil {
		return nil, err
	}
	result := &ContestDetails{Contest: contest}
	if contest.OwnerID != nil {
		owner, err := r.users.GetUserProfileByID(ctx, *contest.OwnerID)
		if err != nil {
			return nil, err
		}
		result.Owner = &owner
	}
	if allowed {
		result.Problems, err = r.contests.GetContestProblems(ctx, contest.ID)
	}
	return result, err
}
// contestProblem returns the contest problem link and whether the caller may
// read its statement.
func (r *DetailsReader) contestProblem(ctx context.Context, org, login string, id uuid.UUID) (contestsmodel.ContestProblem, bool, error) {
	contest, err := r.authorizedContest(ctx, org, login, accessmodel.ActionGetContestProblem)
	if err != nil {
		return contestsmodel.ContestProblem{}, false, err
	}
	link, err := r.contests.GetContestProblem(ctx, contestsmodel.ContestProblemGet{ContestId: contest.ID, ProblemId: id})
	if err != nil {
		return contestsmodel.ContestProblem{}, false, err
	}
	if !contest.GetHideStatements() {
		return link, true, nil
	}
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return link, false, nil
	}
	manager, err := r.permissions.HasContestPermission(ctx, contest.ID, user.Id, accessmodel.ActionManageContest)
	return link, manager, err
}
func (r *DetailsReader) GetContestProblemDetails(ctx context.Context, org, login string, id uuid.UUID) (*ContestProblemDetails, error) {
	link, statementVisible, err := r.contestProblem(ctx, org, login, id)
	if err != nil {
		return nil, err
	}
	problem := problemsmodel.Problem{ID: link.ProblemID, Title: link.Title, ShortName: link.ShortName, TimeLimitMs: link.TimeLimitMs, MemoryLimitMb: link.MemoryLimitMb, CreatedAt: link.CreatedAt, UpdatedAt: link.UpdatedAt}
	result := &ContestProblemDetails{Link: link, ProblemDetails: problemsmodel.ProblemDetails{Problem: problem}, Assets: map[string]string{}}
	if statementVisible {
		content := r.statements.ReadStatement(ctx, id, link.PackageID, "")
		result.Statement, result.Samples, result.Assets = content.Statement, content.Samples, content.Assets
	}
	return result, nil
}

// OpenProblemAsset opens a statement image by content hash from the package
// the contest currently uses for the problem and returns it with its content
// type. It is readable exactly when the statement is.
func (r *DetailsReader) OpenProblemAsset(ctx context.Context, org, login string, id uuid.UUID, sha256 string) (io.ReadCloser, string, error) {
	link, statementVisible, err := r.contestProblem(ctx, org, login, id)
	if err != nil {
		return nil, "", err
	}
	if !statementVisible {
		return nil, "", pkg.Wrap(pkg.NoPermission, nil, "statements are hidden for this contest")
	}
	return r.statements.OpenImage(ctx, link.PackageID, sha256)
}
func (r *DetailsReader) ReadBooklet(ctx context.Context, orgLogin, contestLogin, language string) ([]byte, error) {

	contest, err := r.authorizedContest(ctx, orgLogin, contestLogin, accessmodel.ActionGetContestProblem)
	if err != nil {
		return nil, err
	}

	var isManager bool
	user := accessmodel.UserFromContext(ctx)
	if !user.IsGuest() {
		isManager, err = r.permissions.HasContestPermission(ctx, contest.ID, user.Id, accessmodel.ActionManageContest)
		if err != nil {
			return nil, err
		}
	}

	if !isManager {
		if contest.GetHideStatements() {
			return nil, pkg.Wrap(pkg.NoPermission, nil, "statements are hidden for this contest")
		}
		if contest.StartTime != nil && accessmodel.ReadTime(ctx).Before(*contest.StartTime) {
			return nil, pkg.Wrap(pkg.NoPermission, nil, "contest has not started yet")
		}
	}

	problems, err := r.contests.GetContestProblems(ctx, contest.ID)
	if err != nil {
		return nil, err
	}

	lang := language

	var problemDatas []booklet.ProblemData
	for _, cp := range problems {
		content := r.statements.ReadStatement(ctx, cp.ProblemID, cp.PackageID, lang)
		statement, samples := content.Statement, content.Samples

		title := strings.TrimSpace(cp.Title)
		if statement != nil && strings.TrimSpace(statement.Title) != "" {
			title = strings.TrimSpace(statement.Title)
		}

		var bookletSamples []booklet.SampleData
		for _, s := range samples {
			bookletSamples = append(bookletSamples, booklet.SampleData{
				Input:  s.Input,
				Output: s.Output,
			})
		}

		legend := ""
		inputFormat := ""
		outputFormat := ""
		interaction := ""
		scoring := ""
		notes := ""
		if statement != nil {
			legend = statement.Legend
			inputFormat = statement.InputFormat
			outputFormat = statement.OutputFormat
			interaction = statement.Interaction
			scoring = statement.Scoring
			notes = statement.Notes
		}

		problemDatas = append(problemDatas, booklet.ProblemData{
			Letter:        ordinalToLetter(cp.Ordinal),
			Title:         title,
			TimeLimitMs:   cp.TimeLimitMs,
			MemoryLimitMb: cp.MemoryLimitMb,
			InputFile:     "stdin",
			OutputFile:    "stdout",
			Legend:        legend,
			InputFormat:   inputFormat,
			OutputFormat:  outputFormat,
			Interaction:   interaction,
			Scoring:       scoring,
			Notes:         notes,
			Samples:       bookletSamples,
		})
	}

	dateStr := accessmodel.ReadTime(ctx).Format("02.01.2006")
	if contest.StartTime != nil {
		dateStr = contest.StartTime.Format("02.01.2006")
	}

	contestData := booklet.ContestData{
		Title:        contest.Title,
		Organization: contest.OrganizationLogin,
		Date:         dateStr,
		Language:     lang,
		Problems:     problemDatas,
	}

	texSource, err := booklet.GenerateLatex(contestData)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to generate LaTeX booklet")
	}

	var pdfBytes []byte
	if r.compiler != nil {
		pdfBytes, err = r.compiler.CompilePDF(ctx, texSource)
	} else {
		pdfBytes, err = booklet.CompilePDFLocal(ctx, texSource)
	}
	if err != nil {
		slog.ErrorContext(ctx, "failed to compile PDF booklet", "error", err)
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to compile PDF booklet")
	}

	return pdfBytes, nil
}

func ordinalToLetter(ordinal int) string {
	if ordinal <= 0 {
		return "A"
	}
	res := ""
	for ordinal > 0 {
		rem := (ordinal - 1) % 26
		res = string(rune('A'+rem)) + res
		ordinal = (ordinal - 1) / 26
	}
	return res
}
