package health

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestGetHealth_Success(t *testing.T) {
	app := fiber.New()
	handlers := NewHandlers()

	app.Get("/health", handlers.GetHealth)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response corev1.GetHealthResponseModel
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &response)

	assert.Equal(t, "ok", response.Status)
	assert.Equal(t, "Backend is running", response.Message)
}

func TestGetHealth_ReturnsJSON(t *testing.T) {
	app := fiber.New()
	handlers := NewHandlers()

	app.Get("/health", handlers.GetHealth)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}
