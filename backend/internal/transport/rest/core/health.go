package core

import (
	"context"

	corev1 "github.com/gate149/contracts/core/v1"
)

func (h *CoreServer) GetHealth(ctx context.Context, request corev1.GetHealthRequestObject) (corev1.GetHealthResponseObject, error) {
	return corev1.GetHealth200JSONResponse{
		Status:  "ok",
		Message: "Backend is running",
	}, nil
}
