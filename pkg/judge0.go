package pkg

import (
	"context"
	"fmt"
	"time"

	"github.com/gate149/judge0-go-sdk"
)

// Judge0Client wraps the Judge0 API client
type Judge0Client struct {
	client *judge0.ClientWithResponses
}

// NewJudge0Client creates a new Judge0 client
func NewJudge0Client(baseURL string) (*Judge0Client, error) {
	client, err := judge0.NewClientWithResponses(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create judge0 client: %w", err)
	}

	return &Judge0Client{
		client: client,
	}, nil
}

// MapLanguageID maps Gate language IDs to Judge0 language IDs
// Gate language IDs: Golang=10, Cpp=20, Python=30
func MapLanguageID(gateLanguageID int32) (int, error) {
	switch gateLanguageID {
	case 10: // Golang
		return 60, nil // Go (1.13.5)
	case 20: // Cpp
		return 54, nil // C++ (GCC 9.2.0)
	case 30: // Python
		return 71, nil // Python (3.8.1)
	default:
		return 0, fmt.Errorf("unsupported language: %d", gateLanguageID)
	}
}

// SubmissionResult represents the result of a submission
type SubmissionResult struct {
	Token          string
	Status         *judge0.Status
	Stdout         *string
	Stderr         *string
	CompileOutput  *string
	Time           *string
	Memory         *string
	ExitCode       *int
	Message        *string
}

// CreateSubmission creates a submission in Judge0
func (j *Judge0Client) CreateSubmission(ctx context.Context, sourceCode string, languageID int, stdin string, expectedOutput string, timeLimit float64, memoryLimit int) (*SubmissionResult, error) {
	timeLimitStr := fmt.Sprintf("%.1f", timeLimit)
	memoryLimitStr := fmt.Sprintf("%d", memoryLimit*1024) // Convert MB to KB

	submission := judge0.Submission{
		SourceCode:     sourceCode,
		LanguageId:     languageID,
		Stdin:          &stdin,
		ExpectedOutput: &expectedOutput,
		CpuTimeLimit:   &timeLimitStr,
		MemoryLimit:    &memoryLimitStr,
	}

	waitFlag := false
	resp, err := j.client.CreateSubmissionWithResponse(ctx, &judge0.CreateSubmissionParams{
		Wait: &waitFlag,
	}, submission)
	if err != nil {
		return nil, fmt.Errorf("failed to create submission: %w", err)
	}

	if resp.StatusCode() != 201 {
		if resp.JSON400 != nil {
			return nil, fmt.Errorf("judge0 error: %s", resp.JSON400.Error)
		}
		if resp.JSON503 != nil {
			return nil, fmt.Errorf("judge0 service unavailable: %s", resp.JSON503.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}

	if resp.JSON201 == nil {
		return nil, fmt.Errorf("empty response from judge0")
	}

	return &SubmissionResult{
		Token: resp.JSON201.Token,
	}, nil
}

// GetSubmission retrieves a submission result from Judge0
func (j *Judge0Client) GetSubmission(ctx context.Context, token string) (*SubmissionResult, error) {
	resp, err := j.client.GetSubmissionWithResponse(ctx, token, &judge0.GetSubmissionParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to get submission: %w", err)
	}

	if resp.StatusCode() != 200 {
		if resp.JSON400 != nil {
			return nil, fmt.Errorf("judge0 error: %s", resp.JSON400.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("empty response from judge0")
	}

	sub := resp.JSON200
	return &SubmissionResult{
		Token:         *sub.Token,
		Status:        sub.Status,
		Stdout:        sub.Stdout,
		Stderr:        sub.Stderr,
		CompileOutput: sub.CompileOutput,
		Time:          sub.Time,
		Memory:        sub.Memory,
		ExitCode:      sub.ExitCode,
		Message:       sub.Message,
	}, nil
}

// WaitForSubmission polls Judge0 until the submission is complete
func (j *Judge0Client) WaitForSubmission(ctx context.Context, token string, maxWait time.Duration) (*SubmissionResult, error) {
	deadline := time.Now().Add(maxWait)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("timeout waiting for submission result")
			}

			result, err := j.GetSubmission(ctx, token)
			if err != nil {
				return nil, err
			}

			// Check if submission is complete
			// Status ID 1 = In Queue, 2 = Processing
			if result.Status != nil && result.Status.Id > 2 {
				return result, nil
			}
		}
	}
}

