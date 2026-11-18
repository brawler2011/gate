package pkg

import "time"

type RateLimitError struct{ RetryAfter time.Duration }

func (e *RateLimitError) Error() string { return "rate limit exceeded" }
