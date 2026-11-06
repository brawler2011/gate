//go:build integration
// +build integration

package benchmarks

import (
	"context"
	"fmt"
	"testing"
	"time"

	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
)

func BenchmarkScoreboard_GetContestScoreboardFromStandings(b *testing.B) {
	ctx := context.Background()

	// Seed scoreboard contest with 30 participants and standings results
	contestID := uuid.New()
	_, err := dbPool.Exec(ctx,
		`INSERT INTO contests (id, organization_id, owner_id, visibility, title, login, description, settings)
		 VALUES ($1, $2, $3, 'public', 'Scoreboard Bench Contest', $4, 'desc', '{}')`,
		contestID, testOrgID, testAdminID, "sb_contest_"+uuid.NewString()[:8],
	)
	if err != nil {
		b.Fatalf("failed to create scoreboard contest: %v", err)
	}

	for i := 0; i < 30; i++ {
		uID := uuid.New()
		username := fmt.Sprintf("sb_user_%d_%s", i, uuid.NewString()[:6])
		_, err := dbPool.Exec(ctx,
			`INSERT INTO users (id, username, password_hash, role) VALUES ($1, $2, 'hash', 'user')`,
			uID, username,
		)
		if err != nil {
			b.Fatalf("failed to insert scoreboard user: %v", err)
		}

		_, err = dbPool.Exec(ctx,
			`INSERT INTO contest_members (contest_id, user_id, role) VALUES ($1, $2, 'participant')`,
			contestID, uID,
		)
		if err != nil {
			b.Fatalf("failed to insert contest member: %v", err)
		}

		solved := i%2 == 0
		failedAttempts := int32(i % 5)
		now := time.Now()
		mins := int32(10 + i*2)
		_, err = dbPool.Exec(ctx,
			`INSERT INTO contest_problem_results (contest_id, user_id, problem_id, solved, failed_attempts, first_ac_time, time_minutes)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			contestID, uID, testProblemID, solved, failedAttempts, &now, mins,
		)
		if err != nil {
			b.Fatalf("failed to insert contest problem result: %v", err)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		results, userMap, err := contestsRepo.GetContestScoreboardFromStandings(ctx, contestID)
		if err != nil {
			b.Fatalf("failed to get scoreboard from standings: %v", err)
		}
		if len(results) == 0 || len(userMap) == 0 {
			b.Fatalf("expected results in scoreboard standings")
		}
	}
}

func BenchmarkScoreboard_GetSubmissionsForScoreboard(b *testing.B) {
	ctx := context.Background()

	// Pre-seed submissions for the participant in the test contest
	for i := 0; i < 20; i++ {
		_, err := submissionsRepo.CreateSubmission(ctx, &submissionsmodel.SubmissionCreation{
			ContestId: testContestID,
			ProblemId: testProblemID,
			UserId:    testParticipantID,
			Solution:  "solution",
			Language:  submissionsmodel.Golang,
			Penalty:   0,
		})
		if err != nil {
			b.Fatalf("failed to seed submission for scoreboard: %v", err)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		subs, err := contestsRepo.GetSubmissionsForScoreboard(ctx, testContestID, testParticipantID, testProblemID)
		if err != nil {
			b.Fatalf("failed to get submissions for scoreboard: %v", err)
		}
		if len(subs) == 0 {
			b.Fatalf("expected submissions for scoreboard")
		}
	}
}
