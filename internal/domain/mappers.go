package domain

import (
	"time"

	contestssqlc "github.com/gate149/core/internal/contests/sqlc"
	problemssqlc "github.com/gate149/core/internal/problems/sqlc"
	submissionssqlc "github.com/gate149/core/internal/submissions/sqlc"
	userssqlc "github.com/gate149/core/internal/users/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func UserFromSqlc(u userssqlc.User) User {
	return User{
		ID:        u.ID,
		Username:  u.Username,
		Role:      string(u.Role),
		KratosID:  u.KratosID,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func ContestFromSqlc(c contestssqlc.Contest) Contest {
	return Contest{
		ID:                     c.ID,
		Title:                  c.Title,
		Description:            c.Description,
		Visibility:             string(c.Visibility),
		MonitorScope:           string(c.MonitorScope),
		SubmissionsListScope:   string(c.SubmissionsListScope),
		SubmissionsReviewScope: string(c.SubmissionsReviewScope),
		CreatedBy:              pgtypeToUUID(c.CreatedBy),
		CreatedAt:              c.CreatedAt,
		UpdatedAt:              c.UpdatedAt,
	}
}

func ContestProblemFromSqlc(p contestssqlc.GetContestProblemRow) ContestProblem {
	return ContestProblem{
		ProblemID:        pgtypeToUUID(p.ProblemID),
		Title:            derefString(p.Title),
		TimeLimit:        int64(derefInt32(p.TimeLimit)),
		MemoryLimit:      int64(derefInt32(p.MemoryLimit)),
		Position:         int64(p.Position),
		LegendHtml:       derefString(p.LegendHtml),
		InputFormatHtml:  derefString(p.InputFormatHtml),
		OutputFormatHtml: derefString(p.OutputFormatHtml),
		NotesHtml:        derefString(p.NotesHtml),
		ScoringHtml:      derefString(p.ScoringHtml),
		CreatedAt:        pgtypeToTime(p.CreatedAt),
		UpdatedAt:        pgtypeToTime(p.UpdatedAt),
	}
}

func ContestProblemsListRowFromSqlc(p contestssqlc.GetContestProblemsRow) ContestProblem {
	return ContestProblem{
		ProblemID:   pgtypeToUUID(p.ProblemID),
		Title:       derefString(p.Title),
		TimeLimit:   int64(derefInt32(p.TimeLimit)),
		MemoryLimit: int64(derefInt32(p.MemoryLimit)),
		Position:    int64(p.Position),
		CreatedAt:   pgtypeToTime(p.CreatedAt),
		UpdatedAt:   pgtypeToTime(p.UpdatedAt),
	}
}

func ContestMemberFromSqlc(m contestssqlc.ListContestMembersRow) ContestMember {
	return ContestMember{
		UserID:      pgtypeToUUID(m.UserID),
		ContestID:   m.ContestID,
		Username:    derefString(m.Username),
		Role:        string(m.Role.UserRole),
		ContestRole: string(m.ContestRole),
		KratosID:    derefString(m.KratosID),
		CreatedAt:   pgtypeToTime(m.CreatedAt),
		UpdatedAt:   pgtypeToTime(m.UpdatedAt),
	}
}

func ProblemFromSqlc(p problemssqlc.Problem) Problem {
	return Problem{
		ID:               p.ID,
		CreatedBy:        pgtypeToUUID(p.CreatedBy),
		Visibility:       string(p.Visibility),
		Title:            p.Title,
		TimeLimit:        int64(p.TimeLimit),
		MemoryLimit:      int64(p.MemoryLimit),
		Legend:           p.Legend,
		InputFormat:      p.InputFormat,
		OutputFormat:     p.OutputFormat,
		Notes:            p.Notes,
		Scoring:          p.Scoring,
		LegendHtml:       p.LegendHtml,
		InputFormatHtml:  p.InputFormatHtml,
		OutputFormatHtml: p.OutputFormatHtml,
		NotesHtml:        p.NotesHtml,
		ScoringHtml:      p.ScoringHtml,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
	}
}

func ProblemTestFromSqlc(t problemssqlc.ProblemTest) ProblemTest {
	return ProblemTest{
		ID:        t.ID,
		ProblemID: t.ProblemID,
		Ordinal:   int64(t.Ordinal),
		Input:     t.Input,
		Output:    t.Output,
		CreatedAt: t.CreatedAt,
	}
}

func SubmissionFromSqlc(s submissionssqlc.GetSubmissionRow) Submission {
	return Submission{
		ID:           s.ID,
		CreatedBy:    pgtypeToUUIDPtr(s.CreatedBy),
		Username:     derefString(s.Username),
		Submission:   s.Submission,
		State:        s.State,
		Score:        int64(s.Score),
		Penalty:      int64(s.Penalty),
		TimeStat:     int64(s.TimeStat),
		MemoryStat:   int64(s.MemoryStat),
		Language:     s.Language,
		ProblemID:    pgtypeToUUIDPtr(s.ProblemID),
		ProblemTitle: derefString(s.ProblemTitle),
		Position:     derefInt32ToInt64Ptr(s.Position),
		ContestID:    pgtypeToUUIDPtr(s.ContestID),
		ContestTitle: derefString(s.ContestTitle),
		UpdatedAt:    s.UpdatedAt,
		CreatedAt:    s.CreatedAt,
	}
}

func SubmissionListRowFromSqlc(s submissionssqlc.ListSubmissionsRow) Submission {
	return Submission{
		ID:           s.ID,
		CreatedBy:    pgtypeToUUIDPtr(s.CreatedBy),
		Username:     derefString(s.Username),
		State:        s.State,
		Score:        int64(s.Score),
		Penalty:      int64(s.Penalty),
		TimeStat:     int64(s.TimeStat),
		MemoryStat:   int64(s.MemoryStat),
		Language:     s.Language,
		ProblemID:    pgtypeToUUIDPtr(s.ProblemID),
		ProblemTitle: derefString(s.ProblemTitle),
		Position:     derefInt32ToInt64Ptr(s.Position),
		ContestID:    pgtypeToUUIDPtr(s.ContestID),
		ContestTitle: derefString(s.ContestTitle),
		UpdatedAt:    s.UpdatedAt,
		CreatedAt:    s.CreatedAt,
	}
}

// Helpers

func pgtypeToUUID(uuid pgtype.UUID) uuid.UUID {
	if !uuid.Valid {
		return [16]byte{}
	}
	return uuid.Bytes
}

func pgtypeToUUIDPtr(u pgtype.UUID) *uuid.UUID {
	if !u.Valid {
		return nil
	}
	id := u.Bytes
	return &id
}

func pgtypeToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt32(i *int32) int32 {
	if i == nil {
		return 0
	}
	return *i
}

func derefInt32ToInt64Ptr(i *int32) *int64 {
	if i == nil {
		return nil
	}
	val := int64(*i)
	return &val
}
