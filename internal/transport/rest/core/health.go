package handlers

import (
	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gofiber/fiber/v2"
)

func (h *CoreServer) GetHealth(c *fiber.Ctx) error {
	return c.JSON(&corev1.GetHealthResponseModel{
		Status:  "ok",
		Message: "Backend is running",
	})
}
