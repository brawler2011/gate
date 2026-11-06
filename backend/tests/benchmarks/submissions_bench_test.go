//go:build integration
// +build integration

package benchmarks

import (
	"context"
	"testing"

	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
)

func BenchmarkSubmissions_CreateSubmission(b *testing.B) {
	ctx := context.Background()
	creation := submissionsmodel.SubmissionCreation{
		ContestId: testContestID,
		ProblemId: testProblemID,
		UserId:    testParticipantID,
		Solution:  "package main\nfunc main() {}\n",
		Language:  submissionsmodel.Golang,
		Penalty:   0,
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := submissionsRepo.CreateSubmission(ctx, &creation)
		if err != nil {
			b.Fatalf("failed to create submission: %v", err)
		}
	}
}

func BenchmarkSubmissions_UpdateSubmission(b *testing.B) {
	ctx := context.Background()

	// Pre-create submissions to update
	ids := make([]uuid.UUID, b.N)
	for i := 0; i < b.N; i++ {
		id, err := submissionsRepo.CreateSubmission(ctx, &submissionsmodel.SubmissionCreation{
			ContestId: testContestID,
			ProblemId: testProblemID,
			UserId:    testParticipantID,
			Solution:  "package main\nfunc main() {}\n",
			Language:  submissionsmodel.Golang,
			Penalty:   0,
		})
		if err != nil {
			b.Fatalf("failed to pre-create submission: %v", err)
		}
		ids[i] = id
	}

	update := submissionsmodel.SubmissionUpdate{
		State:      submissionsmodel.Accepted,
		Score:      100,
		TimeStat:   50,
		MemoryStat: 2048,
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := submissionsRepo.UpdateSubmission(ctx, ids[i], &update)
		if err != nil {
			b.Fatalf("failed to update submission: %v", err)
		}
	}
}

func BenchmarkSubmissions_GetSubmission(b *testing.B) {
	ctx := context.Background()

	id, err := submissionsRepo.CreateSubmission(ctx, &submissionsmodel.SubmissionCreation{
		ContestId: testContestID,
		ProblemId: testProblemID,
		UserId:    testParticipantID,
		Solution:  "package main\nfunc main() {}\n",
		Language:  submissionsmodel.Golang,
		Penalty:   0,
	})
	if err != nil {
		b.Fatalf("failed to create submission: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sub, err := submissionsRepo.GetSubmission(ctx, id)
		if err != nil {
			b.Fatalf("failed to get submission: %v", err)
		}
		if sub.ID != id {
			b.Fatalf("mismatched submission id: %v != %v", sub.ID, id)
		}
	}
}

func BenchmarkSubmissions_ListSubmissions(b *testing.B) {
	ctx := context.Background()

	// Ensure submissions exist for contest
	for i := 0; i < 30; i++ {
		_, _ = submissionsRepo.CreateSubmission(ctx, &submissionsmodel.SubmissionCreation{
			ContestId: testContestID,
			ProblemId: testProblemID,
			UserId:    testParticipantID,
			Solution:  "package main\nfunc main() {}\n",
			Language:  submissionsmodel.Golang,
			Penalty:   0,
		})
	}

	filter := submissionsmodel.SubmissionsFilter{
		ContestId: &testContestID,
		Page:      1,
		PageSize:  20,
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		subs, _, _, err := submissionsRepo.ListSubmissions(ctx, filter)
		if err != nil {
			b.Fatalf("failed to list submissions: %v", err)
		}
		if len(subs) == 0 {
			b.Fatalf("expected submissions in list")
		}
	}
}
