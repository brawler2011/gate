package contests

import (
	"context"
	"io"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
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
}
type DetailsReader struct {
	contests   ContestDetailsStore
	statements StatementSource
	access     access.Loader
}

func NewDetailsReader(contests ContestDetailsStore, statements StatementSource, loader access.Loader) *DetailsReader {
	return &DetailsReader{contests: contests, statements: statements, access: loader}
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
	link, a, err := r.problemAccess(ctx, contestID, id)
	return link, a.Can(accessmodel.ContestViewStatements), err
}

func (r *DetailsReader) problemAccess(ctx context.Context, contestID, id uuid.UUID) (contestsmodel.ContestProblem, access.ContestAccess, error) {
	a, err := access.Contest(ctx, r.access, contestID, accessmodel.ContestViewProblems)
	if err != nil {
		return contestsmodel.ContestProblem{}, a, err
	}
	link, err := r.contests.GetContestProblem(ctx, contestsmodel.ContestProblemGet{ContestId: a.Facts.Contest.ID, ProblemId: id})
	return link, a, err
}

func (r *DetailsReader) GetContestProblemDetails(ctx context.Context, contestID, id uuid.UUID) (*ContestProblemDetails, error) {
	link, a, err := r.problemAccess(ctx, contestID, id)
	if err != nil {
		return nil, err
	}
	statementVisible := a.Can(accessmodel.ContestViewStatements)
	problem := problemsmodel.Problem{ID: link.ProblemID, Title: link.Title, ShortName: link.ShortName, TimeLimitMs: link.TimeLimitMs, MemoryLimitMb: link.MemoryLimitMb, CreatedAt: link.CreatedAt, UpdatedAt: link.UpdatedAt}
	result := &ContestProblemDetails{Link: link, ProblemDetails: problemsmodel.ProblemDetails{Problem: problem}, Assets: map[string]string{}}
	if statementVisible {
		content := r.statements.ReadStatement(ctx, id, link.PackageID, "")
		result.Statement, result.Samples, result.Assets = content.Statement, content.Samples, content.Assets
		if result.Statement != nil && !a.Can(accessmodel.ContestViewTutorials) {
			result.Statement.Tutorial = ""
		}
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
