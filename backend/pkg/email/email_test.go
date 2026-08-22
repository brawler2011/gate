package email

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEmailService_Environments(t *testing.T) {
	// 1. Local environment always uses LogEmailService (even if API key is provided)
	svcLocal := NewEmailService("local", "re_test_key", "no-reply@example.com", "http://localhost:3000")
	_, isLogLocal := svcLocal.(*LogEmailService)
	assert.True(t, isLogLocal, "expected LogEmailService for local env")

	// 2. Dev environment with API key uses ResendEmailService
	svcDev := NewEmailService("dev", "re_test_key", "no-reply@example.com", "http://dev.gate.example.com")
	resendDev, isResendDev := svcDev.(*ResendEmailService)
	assert.True(t, isResendDev, "expected ResendEmailService for dev env with API key")
	assert.Equal(t, "re_test_key", resendDev.APIKey)
	assert.Equal(t, "no-reply@example.com", resendDev.FromEmail)

	// 3. Prod environment with API key uses ResendEmailService
	svcProd := NewEmailService("prod", "re_test_key", "Gate <no-reply@gate.example.com>", "https://gate.example.com")
	resendProd, isResendProd := svcProd.(*ResendEmailService)
	assert.True(t, isResendProd, "expected ResendEmailService for prod env with API key")
	assert.Equal(t, "re_test_key", resendProd.APIKey)

	// 4. Non-local without API key falls back to LogEmailService
	svcDevNoKey := NewEmailService("dev", "", "no-reply@example.com", "http://dev.gate.example.com")
	_, isLogDevNoKey := svcDevNoKey.(*LogEmailService)
	assert.True(t, isLogDevNoKey, "expected LogEmailService fallback when API key is empty")
}
