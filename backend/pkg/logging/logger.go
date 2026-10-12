package logging

import (
	"io"
	"log/slog"
)

// New writes sanitized JSON records to a single destination in every environment.
func New(w io.Writer) *slog.Logger {
	return slog.New(NewSanitizingHandler(slog.NewJSONHandler(w, nil)))
}
