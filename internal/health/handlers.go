package health

import (
	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gofiber/fiber/v2"
)

type HealthHandlers struct{}

func NewHandlers() *HealthHandlers {
	return &HealthHandlers{}
}

func (h *HealthHandlers) GetHealth(c *fiber.Ctx) error {
	return c.JSON(&corev1.GetHealthResponseModel{
		Status:  "ok",
		Message: "Backend is running",
	})
}
