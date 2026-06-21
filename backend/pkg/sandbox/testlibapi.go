package sandbox

import (
	"strconv"
	"strings"
)

// Testlib exit codes
const (
	testlibExitOK     = 0
	testlibExitWA     = 1
	testlibExitPE     = 2
	testlibExitPoints = 7
)

// parseTestlibVerdict maps testlib exit status and output to Status and Score.
func parseTestlibVerdict(exitStatus int, stdout, stderr string) (Status, *float64) {
	var status Status
	var score *float64

	switch exitStatus {
	case testlibExitOK:
		status = StatusOK
	case testlibExitWA:
		status = StatusWA
	case testlibExitPE:
		status = StatusPE
	case testlibExitPoints:
		status = StatusPoints
		score = parseScore(stderr)
		if score == nil {
			score = parseScore(stdout)
		}
	default:
		status = StatusFail
	}

	return status, score
}

// parseScore extracts score from checker/interactor output according to testlib API.
func parseScore(output string) *float64 {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil
	}
	var firstLine string
	for line := range strings.Lines(trimmed) {
		firstLine = strings.TrimSpace(line)
		break
	}
	firstLine = strings.TrimSpace(firstLine)
	lower := strings.ToLower(firstLine)

	// points <score> [message]
	if strings.HasPrefix(lower, "points ") {
		fields := strings.Fields(firstLine)
		if len(fields) > 1 {
			if score, err := strconv.ParseFloat(fields[1], 64); err == nil {
				return &score
			}
		}
	}

	// <score> [message]
	fields := strings.Fields(firstLine)
	if len(fields) > 0 {
		if score, err := strconv.ParseFloat(fields[0], 64); err == nil {
			return &score
		}
	}

	return nil
}
