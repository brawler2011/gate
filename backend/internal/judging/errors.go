package judging

import (
	"context"
	"errors"
	"net"

	"github.com/brawler2011/gate/backend/pkg/sandbox"
)

func IsTemporary(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, sandbox.ErrUnavailable) {
		return true
	}
	var network net.Error
	if errors.As(err, &network) {
		return true
	}
	var response interface{ HTTPStatusCode() int }
	return errors.As(err, &response) && (response.HTTPStatusCode() >= 500 || response.HTTPStatusCode() == 429)
}
