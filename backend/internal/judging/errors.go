package judging

import (
	"context"
	"errors"
	"net"
)

func IsTemporary(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var network net.Error
	if errors.As(err, &network) {
		return true
	}
	var response interface{ HTTPStatusCode() int }
	return errors.As(err, &response) && (response.HTTPStatusCode() >= 500 || response.HTTPStatusCode() == 429)
}
