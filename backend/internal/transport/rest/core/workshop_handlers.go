package core

import (
	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// WorkshopHandlers contains workshop-related HTTP handlers
type WorkshopHandlers struct {
	workshopUC interfaces.WorkshopUC
}

// NewWorkshopHandlers creates a new WorkshopHandlers instance
func NewWorkshopHandlers(workshopUC interfaces.WorkshopUC) *WorkshopHandlers {
	return &WorkshopHandlers{
		workshopUC: workshopUC,
	}
}

// InitProblemWorkshop initializes workshop for a problem
func (h *WorkshopHandlers) InitProblemWorkshop(c *fiber.Ctx) error {
	problemIDStr := c.Params("problemId")
	problemID, err := uuid.Parse(problemIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid problem ID",
		})
	}

	// Get problem title from query or use default
	title := c.Query("title", "New Problem")

	if err := h.workshopUC.InitProblemWorkshop(c.Context(), problemID, title); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Workshop initialized successfully",
	})
}

// ListWorkshopFiles lists files in the repository
func (h *WorkshopHandlers) ListWorkshopFiles(c *fiber.Ctx) error {
	problemIDStr := c.Params("problemId")
	problemID, err := uuid.Parse(problemIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid problem ID",
		})
	}

	path := c.Query("path", "")

	files, err := h.workshopUC.ListProblemFiles(c.Context(), problemID, path)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"files": files,
	})
}

// GetWorkshopFile reads a file from the repository
func (h *WorkshopHandlers) GetWorkshopFile(c *fiber.Ctx) error {
	problemIDStr := c.Params("problemId")
	problemID, err := uuid.Parse(problemIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid problem ID",
		})
	}

	path := c.Params("*")

	content, err := h.workshopUC.ReadProblemFile(c.Context(), problemID, path)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	c.Set("Content-Type", "application/octet-stream")
	return c.Send(content)
}

// UpdateWorkshopFile updates a file in the repository
func (h *WorkshopHandlers) UpdateWorkshopFile(c *fiber.Ctx) error {
	problemIDStr := c.Params("problemId")
	problemID, err := uuid.Parse(problemIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid problem ID",
		})
	}

	path := c.Params("*")
	content := c.Body()

	// Get user ID from context (would be set by auth middleware)
	userID := uuid.Nil // TODO: Get from auth context

	req := models.UpdateFileRequest{
		ProblemID: problemID,
		UserID:    userID,
		Path:      path,
		Content:   content,
	}

	if err := h.workshopUC.UpdateProblemFile(c.Context(), req); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "File updated successfully",
	})
}

// DeleteWorkshopFile deletes a file from the repository
func (h *WorkshopHandlers) DeleteWorkshopFile(c *fiber.Ctx) error {
	problemIDStr := c.Params("problemId")
	problemID, err := uuid.Parse(problemIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid problem ID",
		})
	}

	path := c.Params("*")

	if err := h.workshopUC.DeleteProblemFile(c.Context(), problemID, path); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "File deleted successfully",
	})
}

// CommitWorkshopChanges commits changes to the repository
func (h *WorkshopHandlers) CommitWorkshopChanges(c *fiber.Ctx) error {
	problemIDStr := c.Params("problemId")
	problemID, err := uuid.Parse(problemIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid problem ID",
		})
	}

	var req struct {
		Message     string `json:"message"`
		AuthorName  string `json:"author_name"`
		AuthorEmail string `json:"author_email"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.Message == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "commit message is required",
		})
	}

	// Set defaults if not provided
	if req.AuthorName == "" {
		req.AuthorName = "User"
	}

	commitSHA, err := h.workshopUC.CommitChanges(c.Context(), problemID, req.Message, req.AuthorName, req.AuthorEmail)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"commit_sha": commitSHA,
		"message":    "Changes committed successfully",
	})
}

// GetWorkshopStatus gets the current workshop status
func (h *WorkshopHandlers) GetWorkshopStatus(c *fiber.Ctx) error {
	problemIDStr := c.Params("problemId")
	problemID, err := uuid.Parse(problemIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid problem ID",
		})
	}

	status, err := h.workshopUC.GetWorkshopStatus(c.Context(), problemID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(status)
}

// GetWorkshopHistory gets the commit history
func (h *WorkshopHandlers) GetWorkshopHistory(c *fiber.Ctx) error {
	problemIDStr := c.Params("problemId")
	problemID, err := uuid.Parse(problemIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid problem ID",
		})
	}

	limit := c.QueryInt("limit", 20)

	commits, err := h.workshopUC.GetCommitHistory(c.Context(), problemID, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"commits": commits,
	})
}

// CompileProblemComponent compiles a problem component
func (h *WorkshopHandlers) CompileProblemComponent(c *fiber.Ctx) error {
	problemIDStr := c.Params("problemId")
	problemID, err := uuid.Parse(problemIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid problem ID",
		})
	}

	componentType := c.Params("componentType")
	userID := uuid.Nil // TODO: Get from auth context

	req := models.CompileComponentRequest{
		ProblemID:     problemID,
		ComponentType: componentType,
		UserID:        userID,
	}

	result, err := h.workshopUC.CompileProblemComponent(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(result)
}

// GenerateTests generates tests using a generator
func (h *WorkshopHandlers) GenerateTests(c *fiber.Ctx) error {
	problemIDStr := c.Params("problemId")
	problemID, err := uuid.Parse(problemIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid problem ID",
		})
	}

	var reqBody struct {
		GeneratorName string     `json:"generator_name"`
		TestNumbers   []int      `json:"test_numbers"`
		Arguments     [][]string `json:"arguments"`
	}

	if err := c.BodyParser(&reqBody); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	userID := uuid.Nil // TODO: Get from auth context

	req := models.GenerateTestsRequest{
		ProblemID:     problemID,
		GeneratorName: reqBody.GeneratorName,
		TestNumbers:   reqBody.TestNumbers,
		Arguments:     reqBody.Arguments,
		UserID:        userID,
	}

	if err := h.workshopUC.GenerateTests(c.Context(), req); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Tests generated successfully",
	})
}

// ValidateAllTests validates all test inputs
func (h *WorkshopHandlers) ValidateAllTests(c *fiber.Ctx) error {
	problemIDStr := c.Params("problemId")
	problemID, err := uuid.Parse(problemIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid problem ID",
		})
	}

	userID := uuid.Nil // TODO: Get from auth context

	report, err := h.workshopUC.ValidateAllTests(c.Context(), problemID, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(report)
}

// TestSolution tests a solution against tests
func (h *WorkshopHandlers) TestSolution(c *fiber.Ctx) error {
	problemIDStr := c.Params("problemId")
	problemID, err := uuid.Parse(problemIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid problem ID",
		})
	}

	var reqBody struct {
		SolutionPath string `json:"solution_path"`
		TestNumbers  []int  `json:"test_numbers"`
	}

	if err := c.BodyParser(&reqBody); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if reqBody.SolutionPath == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "solution_path is required",
		})
	}

	userID := uuid.Nil // TODO: Get from auth context

	req := models.TestSolutionRequest{
		ProblemID:    problemID,
		SolutionPath: reqBody.SolutionPath,
		TestNumbers:  reqBody.TestNumbers,
		UserID:       userID,
	}

	report, err := h.workshopUC.TestSolution(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(report)
}

// RegisterWorkshopRoutes registers all workshop routes
func RegisterWorkshopRoutes(app *fiber.App, workshopUC interfaces.WorkshopUC) {
	handlers := NewWorkshopHandlers(workshopUC)

	workshop := app.Group("/problems/:problemId/workshop")

	workshop.Post("/init", handlers.InitProblemWorkshop)
	workshop.Get("/files", handlers.ListWorkshopFiles)
	workshop.Get("/files/*", handlers.GetWorkshopFile)
	workshop.Put("/files/*", handlers.UpdateWorkshopFile)
	workshop.Delete("/files/*", handlers.DeleteWorkshopFile)
	workshop.Post("/commit", handlers.CommitWorkshopChanges)
	workshop.Get("/status", handlers.GetWorkshopStatus)
	workshop.Get("/history", handlers.GetWorkshopHistory)
	workshop.Post("/components/:componentType/compile", handlers.CompileProblemComponent)
	workshop.Post("/tests/generate", handlers.GenerateTests)
	workshop.Post("/tests/validate", handlers.ValidateAllTests)
	workshop.Post("/solutions/test", handlers.TestSolution)
}
