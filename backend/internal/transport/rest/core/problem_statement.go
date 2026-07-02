package core

import (
	"context"
	"strings"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg/formats/gfmt"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

func (h *CoreServer) loadProblemStatement(ctx context.Context, problemID uuid.UUID) *models.Statement {
	if h.workshopUC == nil || !h.workshopUC.IsInitialized(ctx, problemID) {
		return nil
	}

	manifest, err := h.workshopUC.GetManifest(ctx, problemID)
	if err != nil {
		return nil
	}

	if hasStatementContent(manifest.Statement) {
		statement := manifest.Statement
		return &statement
	}

	return nil
}

func hasStatementContent(statement models.Statement) bool {
	return strings.TrimSpace(statement.Title) != "" ||
		strings.TrimSpace(statement.Legend) != "" ||
		strings.TrimSpace(statement.InputFormat) != "" ||
		strings.TrimSpace(statement.OutputFormat) != "" ||
		strings.TrimSpace(statement.Notes) != "" ||
		strings.TrimSpace(statement.Scoring) != ""
}

func (h *CoreServer) loadProblemSamples(ctx context.Context, problemID uuid.UUID) []corev1.ProblemSampleModel {
	if h.workshopUC == nil || !h.workshopUC.IsInitialized(ctx, problemID) {
		return []corev1.ProblemSampleModel{}
	}

	yamlBytes, err := h.workshopUC.ReadProblemFile(ctx, problemID, "problem.yaml")
	if err != nil {
		return []corev1.ProblemSampleModel{}
	}

	var prob gfmt.Problem
	if err := yaml.Unmarshal(yamlBytes, &prob); err != nil {
		return []corev1.ProblemSampleModel{}
	}

	samplesSubtask, exists := prob.Subtasks["samples"]
	if !exists {
		return []corev1.ProblemSampleModel{}
	}

	var samples []corev1.ProblemSampleModel
	for _, t := range samplesSubtask.Tests {
		if t.Manual == "" {
			continue
		}

		inputBytes, err := h.workshopUC.ReadProblemFile(ctx, problemID, "tests/"+t.Manual)
		if err != nil {
			continue
		}

		// Try .out first, then .ans
		ansFile := strings.TrimSuffix(t.Manual, ".in") + ".out"
		outputBytes, err := h.workshopUC.ReadProblemFile(ctx, problemID, "tests/"+ansFile)
		if err != nil {
			ansFile = strings.TrimSuffix(t.Manual, ".in") + ".ans"
			outputBytes, err = h.workshopUC.ReadProblemFile(ctx, problemID, "tests/"+ansFile)
		}

		if err != nil {
			continue
		}

		samples = append(samples, corev1.ProblemSampleModel{
			Input:  string(inputBytes),
			Output: string(outputBytes),
		})
	}

	return samples
}

