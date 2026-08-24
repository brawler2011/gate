package core

import (
	"context"

	corev1 "github.com/brawler2011/contracts/core/v1"
)

func (h *CoreServer) GetHealth(ctx context.Context) (*corev1.GetHealthResponseModel, error) {
	return &corev1.GetHealthResponseModel{
		Status:  "ok",
		Message: "Backend is running",
	}, nil
}
