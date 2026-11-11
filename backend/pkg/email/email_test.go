package email

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEmailServiceProvider(t *testing.T) {
	_, logs := NewEmailService(false, "re_test_key", "no-reply@example.com", "http://localhost:3000").(*LogEmailService)
	assert.True(t, logs, "expected LogEmailService when delivery is disabled")

	svc, delivers := NewEmailService(true, "re_test_key", "Gate <no-reply@gate.example.com>", "https://gate.example.com/").(*ResendEmailService)
	assert.True(t, delivers, "expected ResendEmailService when delivery is enabled")
	assert.Equal(t, "https://gate.example.com", svc.AppBaseURL)
}
