package pkg

import "time"

// RateLimitError carries the remaining quota wait to the HTTP error handler.
type RateLimitError struct{ RetryAfter time.Duration }

func (e *RateLimitError) Error() string { return "rate limit exceeded" }
