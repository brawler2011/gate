package middleware

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gate149/core/pkg"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type errorResponse struct {
	Err       string `json:"error"`
	Msg       string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

const (
	requestIDKey contextKey = "request_id"
)

// RequestLoggerMiddleware logs all incoming requests with timing and context
func RequestLoggerMiddleware(logger *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Generate or retrieve request ID
		requestID := c.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("X-Request-ID", requestID)

		// Store requestID in context so it propagates to the entire app
		ctx := context.WithValue(c.UserContext(), requestIDKey, requestID)
		c.SetUserContext(ctx)

		err := c.Next()

		duration := time.Since(start)
		statusCode := c.Response().StatusCode()

		// Build log message
		logMsg := fmt.Sprintf("%s %s -> %d %s (%dms)",
			c.Method(),
			c.Path(),
			statusCode,
			http.StatusText(statusCode),
			duration.Milliseconds(),
		)

		// Build log attributes
		logAttrs := []any{
			slog.String("request_id", requestID),
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Int("status", statusCode),
			slog.Int64("duration_ms", duration.Milliseconds()),
			slog.String("ip", c.IP()),
		}

		if len(c.Queries()) > 0 {
			logAttrs = append(logAttrs, slog.Any("query", c.Queries()))
		}

		// Log based on status code
		if err == nil {
			logLevel := slog.LevelInfo
			if statusCode >= 500 {
				logLevel = slog.LevelError
			} else if statusCode >= 400 {
				logLevel = slog.LevelWarn
			}
			logger.Log(ctx, logLevel, logMsg, logAttrs...)
		}

		return err
	}
}

// ErrorHandlerMiddleware handles errors, maps them to HTTP status codes and logs them
func ErrorHandlerMiddleware(logger *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()
		if err == nil {
			return nil
		}

		// Retrieve Request ID from context
		ctx := c.UserContext()
		requestID, _ := ctx.Value(requestIDKey).(string)
		if requestID == "" {
			// Fallback if context was lost or not set (shouldn't happen with correct middleware order)
			requestID = c.Get("X-Request-ID")
		}

		statusCode := c.Response().StatusCode()

		var cErr *pkg.CustomError
		if errors.As(err, &cErr) {
			statusCode = pkg.ToREST(err)
		}

		resp := errorResponse{
			Err:       http.StatusText(statusCode),
			Msg:       "",
			RequestID: requestID,
		}

		var fErr *fiber.Error
		if errors.As(err, &fErr) {
			statusCode = fErr.Code
			resp.Err = http.StatusText(statusCode)
			resp.Msg = fErr.Message
		}

		logMsg := fmt.Sprintf("%s %s -> %d %s",
			c.Method(),
			c.Path(),
			statusCode,
			http.StatusText(statusCode),
		)

		logAttrs := []any{
			slog.String("request_id", requestID),
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Int("status", statusCode),
			slog.String("error", err.Error()),
			slog.String("ip", c.IP()),
		}

		// Add user/session context if available in the context
		if user, _ := GetUser(ctx); user.ID != uuid.Nil {
			logAttrs = append(logAttrs, slog.String("user_id", user.ID.String()))
		}
		if session, _ := GetSession(ctx); session != nil {
			logAttrs = append(logAttrs, slog.String("session_id", session.Id))
		}

		if cErr != nil {
			resp.Msg = cErr.Message
			logAttrs = append(logAttrs,
				slog.String("operation", cErr.Op),
				slog.String("message", cErr.Message),
			)
			if cErr.Basic != nil {
				logAttrs = append(logAttrs, slog.String("error_type", fmt.Sprintf("%T", cErr.Basic)))
			}
			if cErr.Cause != nil {
				logAttrs = append(logAttrs, slog.String("cause", fmt.Sprintf("%v", cErr.Cause)))
			}
		}

		logLevel := slog.LevelError
		if statusCode < 500 {
			logLevel = slog.LevelWarn
		}
		if statusCode == http.StatusNotFound {
			logLevel = slog.LevelInfo
		}
		logger.Log(ctx, logLevel, logMsg, logAttrs...)

		return c.Status(statusCode).JSON(resp)
	}
}
