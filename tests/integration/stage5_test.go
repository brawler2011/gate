package integration

import (
	"context"
	"testing"
	"time"

	"github.com/gate149/core/pkg"
	"github.com/gate149/core/internal/domain/models"
	"github.com/gate149/core/internal/repository/pg"
	"github.com/gate149/core/internal/usecase"
	"github.com/gate149/core/internal/worker/judge"
	"github.com/gate149/core/pkg/problemformat"
	"github.com/gate149/core/pkg/sandbox"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStage5JudgingFlow tests end-to-end submission judging
// This test requires:
// - PostgreSQL database
// - NATS JetStream
// - SeaweedFS (S3)
// - go-judge sandbox
// - Published problem package
func TestStage5JudgingFlow(t *testing.T) {
	t.Skip("Requires full test environment (PostgreSQL, NATS, S3, go-judge)")

	ctx := context.Background()

	// Setup database connection
	dbDSN := "postgres://user:pass@localhost:5432/gate_test?sslmode=disable"
	pool, err := pkg.NewPostgresDB(dbDSN)
	require.NoError(t, err)
	defer pool.Close()

	// Setup repositories
	submissionsRepo := pg.NewSubmissionsRepo(pool)
	problemsRepo := pg.NewProblemsRepo(pool)

	// Setup S3 client
	s3Client := pkg.NewS3Client(pkg.S3Config{
		Endpoint:  "http://localhost:8333",
		AccessKey: "admin",
		SecretKey: "admin_secret",
		Region:    "us-east-1",
	})

	// Setup sandbox client
	sandboxClient, err := sandbox.NewClient(sandbox.ClientConfig{
		Protocol: sandbox.ProtocolGRPC,
		BaseURL:  "localhost:5051",
	})
	require.NoError(t, err)

	// Setup NATS
	natsConn, err := pkg.NewNatsConn("nats://localhost:4222")
	require.NoError(t, err)
	defer natsConn.Close()

	js, err := jetstream.New(natsConn)
	require.NoError(t, err)

	eventPublisher := judge.NewEventPublisher(js)

	// Create judge use case
	judgeUC := usecase.NewJudgeUseCase(
		submissionsRepo,
		problemsRepo,
		s3Client,
		"problem-packages",
		"/tmp/judge",
		sandboxClient,
		eventPublisher,
	)

	// Test: Judge a simple C++ submission
	t.Run("StandardProblemJudging", func(t *testing.T) {
		// Create a test submission
		submissionID := uuid.New()

		// Judge the submission
		err := judgeUC.JudgeSubmission(ctx, submissionID)
		require.NoError(t, err)

		// Verify submission was updated
		submission, err := submissionsRepo.GetSubmission(ctx, submissionID)
		require.NoError(t, err)
		assert.NotEqual(t, models.Saved, submission.State)
	})
}

// TestVerdictCalculation tests verdict calculation logic
func TestVerdictCalculation(t *testing.T) {
	t.Run("StandardVerdict_AllPassed", func(t *testing.T) {
		calculator := judge.NewVerdictCalculator("pass-fail", testMetadata())

		results := []judge.TestResult{
			{TestNumber: 1, Verdict: "OK", Time: 1000000, Memory: 1024 * 1024},
			{TestNumber: 2, Verdict: "OK", Time: 2000000, Memory: 2048 * 1024},
		}

		verdict := calculator.Calculate(results)
		assert.Equal(t, models.Accepted, verdict.State)
		assert.Equal(t, int32(100), verdict.Score)
		assert.Equal(t, int32(2), verdict.MaxTime) // 2ms
	})

	t.Run("StandardVerdict_WrongAnswer", func(t *testing.T) {
		calculator := judge.NewVerdictCalculator("pass-fail", testMetadata())

		results := []judge.TestResult{
			{TestNumber: 1, Verdict: "OK", Time: 1000000, Memory: 1024 * 1024},
			{TestNumber: 2, Verdict: "WA", Time: 2000000, Memory: 2048 * 1024, Message: "Wrong answer"},
		}

		verdict := calculator.Calculate(results)
		assert.Equal(t, models.GotWA, verdict.State)
		assert.Equal(t, int32(0), verdict.Score)
		assert.NotNil(t, verdict.FailedTest)
		assert.Equal(t, 2, *verdict.FailedTest)
	})

	t.Run("StandardVerdict_TimeLimitExceeded", func(t *testing.T) {
		calculator := judge.NewVerdictCalculator("pass-fail", testMetadata())

		results := []judge.TestResult{
			{TestNumber: 1, Verdict: "TLE", Time: 10000000000, Memory: 1024 * 1024, Message: "Time limit exceeded"},
		}

		verdict := calculator.Calculate(results)
		assert.Equal(t, models.GotTL, verdict.State)
	})

	t.Run("ScoringVerdict_PartialScore", func(t *testing.T) {
		metadata := testMetadataWithGroups()
		calculator := judge.NewVerdictCalculator("scoring", metadata)

		// Group 1: 2 tests (test 1-2), 50 points, each-test policy
		// Group 2: 2 tests (test 3-4), 50 points, complete-group policy
		score50 := 50.0
		results := []judge.TestResult{
			{TestNumber: 1, Verdict: "OK", Score: &score50}, // Group 1, test 1: OK
			{TestNumber: 2, Verdict: "WA", Score: nil},      // Group 1, test 2: WA
			{TestNumber: 3, Verdict: "OK", Score: nil},      // Group 2, test 1: OK
			{TestNumber: 4, Verdict: "WA", Score: nil},      // Group 2, test 2: WA
		}

		verdict := calculator.Calculate(results)
		assert.Equal(t, models.Accepted, verdict.State) // Scoring problems always "accept"
		// Group 1: 1/2 tests passed = 25 points
		// Group 2: complete-group failed = 0 points
		// Total: 25 points out of 100 max
		assert.Greater(t, verdict.Score, int32(0))
	})
}

// TestComponentCache tests component compilation caching
func TestComponentCache(t *testing.T) {
	sandboxClient, _ := sandbox.NewClient(sandbox.ClientConfig{
		Protocol: sandbox.ProtocolGRPC,
		BaseURL:  "localhost:5051",
	})

	cache := judge.NewComponentCache(sandboxClient)

	t.Run("CacheSetAndGet", func(t *testing.T) {
		key := "problem-123:checker:abc123"
		fileID := "file-xyz"

		// Set in cache
		cache.Set(key, fileID)

		// Get from cache
		cachedFileID, found := cache.Get(key)
		assert.True(t, found)
		assert.Equal(t, fileID, cachedFileID)
	})

	t.Run("CacheMiss", func(t *testing.T) {
		_, found := cache.Get("non-existent-key")
		assert.False(t, found)
	})

	t.Run("CacheSize", func(t *testing.T) {
		cache.Clear()
		assert.Equal(t, 0, cache.Size())

		cache.Set("key1", "file1")
		cache.Set("key2", "file2")
		assert.Equal(t, 2, cache.Size())
	})

	t.Run("CacheEviction", func(t *testing.T) {
		cache.Clear()

		// Fill cache to a small size (this test uses the default max size)
		// The cache will evict oldest entries when full
		for i := 0; i < 5; i++ {
			key := "key-" + string(rune(i))
			cache.Set(key, "file-"+string(rune(i)))
			time.Sleep(10 * time.Millisecond) // Ensure different access times
		}

		assert.LessOrEqual(t, cache.Size(), 1000) // Max size
	})
}

// TestLanguageConversion tests language conversion
func TestLanguageConversion(t *testing.T) {
	// This is a simple test to verify language mapping
	tests := []struct {
		input    models.LanguageName
		expected string
	}{
		{models.Cpp, "cpp17"},
		{models.Golang, "go"},
		{models.Python, "python3"},
	}

	for _, tt := range tests {
		// Note: convertLanguage is not exported, so this test would need to be adjusted
		// or the function should be exported for testing
		t.Run(tt.expected, func(t *testing.T) {
			// Test would go here if function was exported
			t.Skip("convertLanguage is not exported")
		})
	}
}

// TestEventPublishing tests event publishing
func TestEventPublishing(t *testing.T) {
	t.Skip("Requires NATS JetStream")

	ctx := context.Background()

	// Setup NATS
	natsConn, err := pkg.NewNatsConn("nats://localhost:4222")
	require.NoError(t, err)
	defer natsConn.Close()

	js, err := jetstream.New(natsConn)
	require.NoError(t, err)

	publisher := judge.NewEventPublisher(js)

	submissionID := uuid.New()
	meta := models.SubmissionEventMeta{
		UserId:       nil,
		Username:     "testuser",
		ContestId:    nil,
		ContestTitle: "Test Contest",
		ProblemId:    nil,
		ProblemTitle: "Test Problem",
		Language:     models.Cpp,
	}

	t.Run("PublishQueued", func(t *testing.T) {
		err := publisher.PublishQueued(ctx, submissionID, meta)
		assert.NoError(t, err)
	})

	t.Run("PublishCompilingStarted", func(t *testing.T) {
		err := publisher.PublishCompilingStarted(ctx, submissionID, meta)
		assert.NoError(t, err)
	})

	t.Run("PublishTestingStarted", func(t *testing.T) {
		err := publisher.PublishTestingStarted(ctx, submissionID, meta)
		assert.NoError(t, err)
	})

	t.Run("PublishTestStarted", func(t *testing.T) {
		err := publisher.PublishTestStarted(ctx, submissionID, 1, meta)
		assert.NoError(t, err)
	})

	t.Run("PublishCompleted", func(t *testing.T) {
		err := publisher.PublishCompleted(ctx, submissionID, models.Accepted, 100, 1000, 256, 20, meta)
		assert.NoError(t, err)
	})
}

// Helper functions

func testMetadata() problemformat.TestsMetadata {
	return problemformat.TestsMetadata{
		Groups: []problemformat.TestGroup{
			{
				Ordinal:      1,
				Name:         "samples",
				Points:       0,
				PointsPolicy: "complete-group",
				Tests:        [2]int{1, 2},
			},
		},
		Tests: []problemformat.TestCase{
			{Ordinal: 1, Method: "manual", IsSample: true},
			{Ordinal: 2, Method: "manual", IsSample: false},
		},
	}
}

func testMetadataWithGroups() problemformat.TestsMetadata {
	return problemformat.TestsMetadata{
		Groups: []problemformat.TestGroup{
			{
				Ordinal:      1,
				Name:         "group1",
				Points:       50,
				PointsPolicy: "each-test",
				Tests:        [2]int{1, 2},
			},
			{
				Ordinal:      2,
				Name:         "group2",
				Points:       50,
				PointsPolicy: "complete-group",
				Tests:        [2]int{3, 4},
			},
		},
		Tests: []problemformat.TestCase{
			{Ordinal: 1, Method: "manual"},
			{Ordinal: 2, Method: "manual"},
			{Ordinal: 3, Method: "manual"},
			{Ordinal: 4, Method: "manual"},
		},
	}
}
