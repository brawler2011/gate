//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/google/uuid"
)

// rateClock is the limiter's clock; the zero time means the real one.
type rateClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *rateClock) set(at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = at
}

func (c *rateClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.at.IsZero() {
		return time.Now()
	}
	return c.at
}

func (s *IntegrationTestSuite) requireRateLimited(err error) {
	s.T().Helper()
	s.Require().Error(err)
	s.Require().Equal(http.StatusTooManyRequests, s.getStatusCode(err))
	seconds, parseErr := strconv.Atoi(s.transport.lastResponse.Header.Get("Retry-After"))
	s.Require().NoError(parseErr)
	s.Positive(seconds)
}

func (s *IntegrationTestSuite) TestRateLimitedLogin() {
	name := "limited_login"
	password := "Password12345"
	registered, err := s.client.Register(s.ctx, &corev1.RegisterRequestModel{Username: name, Email: name + "@example.com", Password: password})
	s.Require().NoError(err)
	_, err = s.dbPool.Exec(s.ctx, "UPDATE users SET is_email_verified = TRUE WHERE id = $1", registered.User.ID)
	s.Require().NoError(err)
	identifier := name
	login := func(password string) error {
		_, err := s.client.Login(s.ctx, &corev1.LoginRequestModel{Identifier: identifier, Password: password})
		return err
	}
	for attempt := range 15 {
		identifier = name
		if attempt%2 == 0 {
			identifier = strings.ToUpper(name)
		}
		s.Equal(http.StatusUnauthorized, s.getStatusCode(login("wrong-password")))
	}
	identifier = name
	s.requireRateLimited(login("wrong-password"))
	s.Equal("120", s.transport.lastResponse.Header.Get("Retry-After"))
	// Correct credentials with different casing must clear the same failure key.
	identifier = strings.ToUpper(name)
	s.Require().NoError(login(password))
	for range 15 {
		s.Equal(http.StatusUnauthorized, s.getStatusCode(login("wrong-password")))
	}
	s.requireRateLimited(login("wrong-password"))
	s.rateClock.set(time.Now().Add(2 * time.Minute))
	s.Equal(http.StatusUnauthorized, s.getStatusCode(login("wrong-password")))
	// Unverified accounts are rejected after the password check and do not consume failure quota.
	other, err := s.client.Register(s.ctx, &corev1.RegisterRequestModel{Username: "unverified_limit", Email: "unverified_limit@example.com", Password: password})
	s.Require().NoError(err)
	_, err = s.client.Login(s.ctx, &corev1.LoginRequestModel{Identifier: other.User.Username, Password: password})
	s.Equal(http.StatusForbidden, s.getStatusCode(err))
}

func (s *IntegrationTestSuite) TestRateLimitedEmail() {
	for _, endpoint := range []string{"register", "forgot-password"} {
		s.Run(endpoint, func() {
			name := "email_limit_" + uuid.NewString()[:8]
			email := name + "@example.com"
			register := func() error {
				_, err := s.client.Register(s.ctx, &corev1.RegisterRequestModel{Username: name, Email: email, Password: "Password12345"})
				return err
			}
			s.Require().NoError(register())
			send := register
			if endpoint == "forgot-password" {
				send = func() error {
					return s.client.ForgotPassword(s.ctx, &corev1.ForgotPasswordRequestModel{Identifier: name})
				}
			}
			// Register and password reset share the recipient's budget.
			for range 4 {
				s.Require().NoError(send())
			}
			s.requireRateLimited(send())
			// Moving through short windows still enforces hourly and daily quotas.
			now := time.Now()
			for batch := 1; batch < 4; batch++ {
				s.rateClock.set(now.Add(time.Duration(batch) * 5 * time.Minute))
				for range 5 {
					s.Require().NoError(send())
				}
			}
			s.rateClock.set(now.Add(20 * time.Minute))
			s.requireRateLimited(send())
			s.rateClock.set(now.Add(time.Hour))
			for batch := 0; batch < 4; batch++ {
				s.rateClock.set(now.Add(time.Hour + time.Duration(batch)*5*time.Minute))
				for range 5 {
					s.Require().NoError(send())
				}
			}
			s.rateClock.set(now.Add(2 * time.Hour))
			for batch := 0; batch < 2; batch++ {
				s.rateClock.set(now.Add(2*time.Hour + time.Duration(batch)*5*time.Minute))
				for range 5 {
					s.Require().NoError(send())
				}
			}
			s.rateClock.set(now.Add(3 * time.Hour))
			s.requireRateLimited(send())
			s.rateClock.set(now.Add(24 * time.Hour))
			s.Require().NoError(send())
			s.rateClock.set(time.Time{})
		})
	}
}

func (s *IntegrationTestSuite) TestRateLimitedSubmissions() {
	user := s.createUser("limited_submitter", accessmodel.UserRoleUser)
	other := s.createUser("other_submitter", accessmodel.UserRoleUser)
	// Exhausted quotas return 429 before a missing problem can reach the handler.
	send := func(id uuid.UUID) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/submissions?problem_id="+uuid.NewString()+"&contest_id="+uuid.NewString()+"&language=python3", strings.NewReader(`{"submission":"print(1)"}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Test-User-ID", id.String())
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, r)
		return rr
	}
	now := time.Now()
	s.rateClock.set(now)
	for batch := 0; batch < 5; batch++ {
		s.rateClock.set(now.Add(time.Duration(batch) * 5 * time.Minute))
		for range 60 {
			s.Require().Equal(http.StatusNotFound, send(user.Id).Code)
		}
		rr := send(user.Id)
		s.Require().Equal(http.StatusTooManyRequests, rr.Code)
		s.NotEmpty(rr.Header().Get("Retry-After"))
		var body map[string]any
		s.Require().NoError(json.Unmarshal(rr.Body.Bytes(), &body))
		s.Equal("rate_limited", body["code"])
	}
	s.rateClock.set(now.Add(25 * time.Minute))
	s.Equal(http.StatusTooManyRequests, send(user.Id).Code)
	s.Equal(http.StatusNotFound, send(other.Id).Code)
	s.rateClock.set(now.Add(time.Hour))
	s.Equal(http.StatusNotFound, send(user.Id).Code)
}

func (s *IntegrationTestSuite) TestRateLimitedProxyClients() {
	send := func(peer, forwarded string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/health", nil)
		r.RemoteAddr = peer + ":1234"
		r.Header.Set("X-Forwarded-For", forwarded)
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, r)
		return rr
	}
	for attempt := range 300 {
		// A changing attacker-controlled prefix must not change the selected IP.
		s.Require().Equal(http.StatusOK, send("192.0.2.254", fmt.Sprintf("198.51.100.%d, 203.0.113.1", attempt%255)).Code)
	}
	blocked := send("192.0.2.254", "203.0.113.1")
	s.Equal(http.StatusTooManyRequests, blocked.Code)
	s.NotEmpty(blocked.Header().Get("Retry-After"))
	s.Equal(http.StatusOK, send("192.0.2.254", "203.0.113.2").Code)
	for attempt := range 300 {
		s.Require().Equal(http.StatusOK, send("192.0.2.3", fmt.Sprintf("198.51.100.%d", attempt%255)).Code)
	}
	s.Equal(http.StatusTooManyRequests, send("192.0.2.3", "203.0.113.2").Code)
}
