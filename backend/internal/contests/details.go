package contests

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/internal/problems"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/statement"
	"github.com/google/uuid"
)

type ContestDetails struct {
	Contest  contestsmodel.Contest
	Problems []contestsmodel.ContestProblem
	// The caller's role, pending join request and virtual run; empty for guests.
	Role        accessmodel.ContestRole
	JoinRequest *contestsmodel.ContestJoinRequest
	VirtualRun  *contestsmodel.VirtualRun
}
type ContestProblemDetails struct {
	Link contestsmodel.ContestProblem
	problemsmodel.ProblemDetails
	Assets map[string]string
}
type ContestDetailsStore interface {
	GetContestByOrgLoginAndContestLogin(context.Context, string, string) (contestsmodel.Contest, error)
	GetContestProblem(context.Context, contestsmodel.ContestProblemGet) (contestsmodel.ContestProblem, error)
	GetContestProblems(context.Context, uuid.UUID) ([]contestsmodel.ContestProblem, error)
	GetPendingContestJoinRequest(ctx context.Context, contestID, userID uuid.UUID) (*contestsmodel.ContestJoinRequest, error)
}
type StatementSource interface {
	ReadStatement(context.Context, uuid.UUID, uuid.UUID, string) problemsmodel.StatementContent
	OpenImage(context.Context, uuid.UUID, string) (io.ReadCloser, string, error)
	OpenPDF(ctx context.Context, problemID, packageID uuid.UUID, language string) (io.ReadCloser, error)
	ReadPrinted(ctx context.Context, problemID, packageID uuid.UUID, language string) (problems.Printed, error)
}
type Typesetter interface {
	PDF(ctx context.Context, doc string, files map[string][]byte) ([]byte, error)
}
type DetailsReader struct {
	contests   ContestDetailsStore
	statements StatementSource
	access     access.Loader
	typeset    Typesetter
}

func NewDetailsReader(contests ContestDetailsStore, statements StatementSource, loader access.Loader, typeset Typesetter) *DetailsReader {
	return &DetailsReader{contests: contests, statements: statements, access: loader, typeset: typeset}
}

// GetContestDetails resolves the contest by logins; every other contest route takes its id.
func (r *DetailsReader) GetContestDetails(ctx context.Context, org, login string) (*ContestDetails, error) {
	contest, err := r.contests.GetContestByOrgLoginAndContestLogin(ctx, org, login)
	if err != nil {
		return nil, err
	}
	a, err := access.Contest(ctx, r.access, contest.ID, accessmodel.ContestView)
	if err != nil {
		return nil, err
	}
	result := &ContestDetails{Contest: a.Facts.Contest, Role: a.Role(), VirtualRun: a.Facts.Run}
	if a.Can(accessmodel.ContestViewProblems) {
		if result.Problems, err = r.contests.GetContestProblems(ctx, contest.ID); err != nil {
			return nil, err
		}
	}
	if a.Actor.Id != uuid.Nil {
		if result.JoinRequest, err = r.contests.GetPendingContestJoinRequest(ctx, contest.ID, a.Actor.Id); err != nil {
			return nil, err
		}
	}
	return result, nil
}
func (r *DetailsReader) contestProblem(ctx context.Context, contestID, id uuid.UUID) (contestsmodel.ContestProblem, bool, error) {
	a, err := access.Contest(ctx, r.access, contestID, accessmodel.ContestViewProblems)
	if err != nil {
		return contestsmodel.ContestProblem{}, false, err
	}
	link, err := r.contests.GetContestProblem(ctx, contestsmodel.ContestProblemGet{ContestId: a.Facts.Contest.ID, ProblemId: id})
	return link, a.Can(accessmodel.ContestViewStatements), err
}
func (r *DetailsReader) GetContestProblemDetails(ctx context.Context, contestID, id uuid.UUID) (*ContestProblemDetails, error) {
	link, statementVisible, err := r.contestProblem(ctx, contestID, id)
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

// Statement assets have the same visibility as their statement.
func (r *DetailsReader) OpenProblemAsset(ctx context.Context, contestID, id uuid.UUID, sha256 string) (io.ReadCloser, string, error) {
	link, statementVisible, err := r.contestProblem(ctx, contestID, id)
	if err != nil {
		return nil, "", err
	}
	if !statementVisible {
		return nil, "", pkg.Wrap(pkg.ErrStatementsHidden, nil, "statements are hidden for this contest")
	}
	return r.statements.OpenImage(ctx, link.PackageID, sha256)
}
// OpenProblemPDF opens the statement PDF of the package the contest uses; it has the statement's visibility.
func (r *DetailsReader) OpenProblemPDF(ctx context.Context, contestID, id uuid.UUID, language string) (io.ReadCloser, error) {
	link, statementVisible, err := r.contestProblem(ctx, contestID, id)
	if err != nil {
		return nil, err
	}
	if !statementVisible {
		return nil, pkg.Wrap(pkg.ErrStatementsHidden, nil, "statements are hidden for this contest")
	}
	return r.statements.OpenPDF(ctx, link.ProblemID, link.PackageID, language)
}

func (r *DetailsReader) ReadBooklet(ctx context.Context, contestID uuid.UUID, language string) ([]byte, error) {
	a, err := access.Contest(ctx, r.access, contestID, accessmodel.ContestView)
	if err != nil {
		return nil, err
	}
	contest := a.Facts.Contest
	if !a.Can(accessmodel.ContestViewProblems) {
		if contest.StartTime != nil && a.Facts.Now.Before(*contest.StartTime) {
			return nil, pkg.Wrap(pkg.ErrContestNotStarted, nil, "contest has not started yet")
		}
		return nil, pkg.ErrForbidden
	}
	if !a.Can(accessmodel.ContestViewStatements) {
		return nil, pkg.Wrap(pkg.ErrStatementsHidden, nil, "statements are hidden for this contest")
	}
	links, err := r.contests.GetContestProblems(ctx, contest.ID)
	if err != nil {
		return nil, err
	}
	date := accessmodel.ReadTime(ctx)
	if contest.StartTime != nil {
		date = *contest.StartTime
	}
	booklet := statement.Contest{Title: contest.Title, Place: contest.OrganizationLogin, Date: date.Format("02.01.2006"), Language: language}
	files := map[string][]byte{"olymp.sty": statement.OlympSty}
	for i, cp := range links {
		printed, err := r.statements.ReadPrinted(ctx, cp.ProblemID, cp.PackageID, language)
		if err != nil {
			return nil, err
		}
		limits := problemsmodel.PrintedLimits{TimeLimitMs: cp.TimeLimitMs, MemoryLimitMb: cp.MemoryLimitMb, InputFile: cp.InputFile, OutputFile: cp.OutputFile}
		p := problemsmodel.PrintedProblem(cp.Title, limits, printed.Statement, printed.Samples)
		// Problems may show different images under one name.
		p.Letter, p.Assets = ordinalToLetter(cp.Ordinal), fmt.Sprintf("assets/%d", i)
		for name, data := range printed.Images {
			files[p.Assets+"/"+name] = data
		}
		booklet.Problems = append(booklet.Problems, p)
	}
	pdf, err := r.typeset.PDF(ctx, statement.NewDocument(booklet).Source, files)
	if err != nil {
		slog.ErrorContext(ctx, "failed to compile PDF booklet", "error", err)
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to compile PDF booklet")
	}
	return pdf, nil
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
