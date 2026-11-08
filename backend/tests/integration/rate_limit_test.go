//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/google/uuid"
)

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
	login := func(password string) error {
		_, err := s.client.Login(s.ctx, &corev1.LoginRequestModel{Identifier: name, Password: password})
		return err
	}
	for range 15 {
		s.Equal(http.StatusUnauthorized, s.getStatusCode(login("wrong-password")))
	}
	s.requireRateLimited(login("wrong-password"))
	s.Equal("120", s.transport.lastResponse.Header.Get("Retry-After"))
	// Correct credentials must work even during cooldown and clear failures.
	s.Require().NoError(login(password))
	s.False(s.rateRedis.Exists("rl:fail:login:" + name))
	for range 15 {
		s.Equal(http.StatusUnauthorized, s.getStatusCode(login("wrong-password")))
	}
	s.requireRateLimited(login("wrong-password"))
	s.rateRedis.FastForward(2 * time.Minute)
	s.Equal(http.StatusUnauthorized, s.getStatusCode(login("wrong-password")))
	// Unverified accounts are BadInput and do not consume failure quota.
	other, err := s.client.Register(s.ctx, &corev1.RegisterRequestModel{Username: "unverified_limit", Email: "unverified_limit@example.com", Password: password})
	s.Require().NoError(err)
	_, err = s.client.Login(s.ctx, &corev1.LoginRequestModel{Identifier: other.User.Username, Password: password})
	s.Equal(http.StatusBadRequest, s.getStatusCode(err))
	s.False(s.rateRedis.Exists("rl:fail:login:" + other.User.Username))
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
				s.rateRedis.SetTime(now.Add(time.Duration(batch) * 5 * time.Minute))
				for range 5 {
					s.Require().NoError(send())
				}
			}
			s.rateRedis.SetTime(now.Add(20 * time.Minute))
			s.requireRateLimited(send()) // 20/hour
			s.rateRedis.SetTime(now.Add(time.Hour))
			for batch := 0; batch < 4; batch++ {
				s.rateRedis.SetTime(now.Add(time.Hour + time.Duration(batch)*5*time.Minute))
				for range 5 {
					s.Require().NoError(send())
				}
			}
			s.rateRedis.SetTime(now.Add(2 * time.Hour))
			for batch := 0; batch < 2; batch++ {
				s.rateRedis.SetTime(now.Add(2*time.Hour + time.Duration(batch)*5*time.Minute))
				for range 5 {
					s.Require().NoError(send())
				}
			}
			s.rateRedis.SetTime(now.Add(3 * time.Hour))
			s.requireRateLimited(send()) // 50/day
			s.rateRedis.SetTime(now.Add(24 * time.Hour))
			s.Require().NoError(send())
			s.rateRedis.SetTime(time.Now())
		})
	}
}

func (s *IntegrationTestSuite) TestRateLimitedSubmissions() {
	user := s.createUser("limited_submitter", accessmodel.UserRoleUser)
	other := s.createUser("other_submitter", accessmodel.UserRoleUser)
	// A nonexistent problem reaches the real handler and returns 404 within quota;
	// once exhausted, middleware must return 429 before the handler runs.
	send := func(id uuid.UUID) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/submissions?problem_id="+uuid.NewString()+"&organization_login=missing&contest_login=missing&language=30", strings.NewReader(`{"submission":"print(1)"}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Test-User-ID", id.String())
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, r)
		return rr
	}
	now := time.Now()
	s.rateRedis.SetTime(now)
	for batch := 0; batch < 5; batch++ {
		s.rateRedis.SetTime(now.Add(time.Duration(batch) * 5 * time.Minute))
		for range 60 {
			s.Require().Equal(http.StatusNotFound, send(user.Id).Code)
		}
		rr := send(user.Id)
		s.Require().Equal(http.StatusTooManyRequests, rr.Code)
		s.NotEmpty(rr.Header().Get("Retry-After"))
		var body map[string]any
		s.Require().NoError(json.Unmarshal(rr.Body.Bytes(), &body))
		s.Equal("Too Many Requests", body["error"])
	}
	s.rateRedis.SetTime(now.Add(25 * time.Minute))
	s.Equal(http.StatusTooManyRequests, send(user.Id).Code) // 300/hour
	s.Equal(http.StatusNotFound, send(other.Id).Code)
	s.rateRedis.SetTime(now.Add(time.Hour))
	s.Equal(http.StatusNotFound, send(user.Id).Code)
	s.rateRedis.Close()
	s.Equal(http.StatusNotFound, send(user.Id).Code)
}

func (s *IntegrationTestSuite) TestRateLimitRedisOutage() {
	s.rateRedis.Close()
	// Neither anonymous middleware nor login failures may turn an outage into 500.
	_, err := s.client.Login(context.Background(), &corev1.LoginRequestModel{Identifier: "missing", Password: "wrong-password"})
	s.Equal(http.StatusUnauthorized, s.getStatusCode(err))
	_, err = s.client.Register(s.ctx, &corev1.RegisterRequestModel{Username: "redis_outage", Email: "redis_outage@example.com", Password: "Password12345"})
	s.Require().NoError(err)
	s.Require().NoError(s.client.ForgotPassword(s.ctx, &corev1.ForgotPasswordRequestModel{Identifier: "redis_outage"}))
}
