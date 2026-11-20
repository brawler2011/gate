package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/ogen-go/ogen/ogenerrors"
)

// NewProblem maps err to the response body and attaches err to the request log line.
func NewProblem(ctx context.Context, err error) *corev1.ProblemStatusCodeWithHeaders {
	if state, ok := ctx.Value(requestLogKey{}).(*requestLog); ok {
		state.err = errors.Join(err, state.err)
	}
	return newProblem(ctx, err)
}

func newProblem(ctx context.Context, err error) *corev1.ProblemStatusCodeWithHeaders {
	code := pkg.CodeOf(err)
	status := pkg.HTTPStatus(code)
	detail := ""
	res := &corev1.ProblemStatusCodeWithHeaders{}

	var appErr *pkg.Error
	var rateErr *pkg.RateLimitError
	switch {
	case errors.Is(err, context.Canceled) && ctx.Err() != nil:
		code, status = pkg.CodeInternal, statusClientClosedRequest
	case errors.As(err, &appErr):
		detail = appErr.Message
		res.Response.Params = problemParams(appErr.Params)
	case errors.As(err, &rateErr):
		detail = rateErr.Error()
		res.RetryAfter = corev1.NewOptInt(int(max(1, (rateErr.RetryAfter+time.Second-1)/time.Second)))
	case code == pkg.CodePayloadTooLarge:
		detail = "request body exceeds maximum allowed size"
	default:
		// ogen's decode and security errors are client errors with no pkg code.
		if s := ogenerrors.ErrorCode(err); s != http.StatusInternalServerError {
			status, code = s, statusCode(s)
			detail = err.Error()
		}
	}
	if status >= 500 || status == statusClientClosedRequest {
		detail = ""
	}

	res.StatusCode = status
	params := res.Response.Params
	res.Response = corev1.Problem{
		Type:      "about:blank",
		Title:     http.StatusText(status),
		Status:    status,
		Code:      corev1.ErrorCode(code),
		Params:    params,
		RequestID: requestid.Get(ctx),
	}
	if detail != "" {
		res.Response.Detail = corev1.NewOptString(detail)
	}
	var fields pkg.FieldErrors
	if errors.As(err, &fields) {
		for _, f := range fields {
			res.Response.Errors = append(res.Response.Errors, corev1.FieldProblem{Pointer: f.Pointer, Code: corev1.ErrorCodeInvalid, Detail: corev1.NewOptString(f.Message)})
		}
	}
	return res
}

func problemParams(params map[string]any) corev1.OptProblemParams {
	if len(params) == 0 {
		return corev1.OptProblemParams{}
	}
	out := make(corev1.ProblemParams, len(params))
	for k, v := range params {
		raw, err := json.Marshal(v)
		if err != nil {
			continue
		}
		out[k] = raw
	}
	return corev1.NewOptProblemParams(out)
}

func statusCode(status int) pkg.Code {
	switch status {
	case http.StatusBadRequest:
		return pkg.CodeInvalid
	case http.StatusUnauthorized:
		return pkg.CodeUnauthenticated
	case http.StatusForbidden:
		return pkg.CodeForbidden
	case http.StatusNotFound:
		return pkg.CodeNotFound
	case http.StatusConflict:
		return pkg.CodeConflict
	case http.StatusRequestEntityTooLarge:
		return pkg.CodePayloadTooLarge
	case http.StatusTooManyRequests:
		return pkg.CodeRateLimited
	case http.StatusNotImplemented:
		return pkg.CodeNotImplemented
	}
	return pkg.CodeInternal
}

// ResponseErrorHandler serves errors raised outside operation handlers: decoding, security and middleware.
func ResponseErrorHandler(logger *slog.Logger) func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
		if ctx == nil {
			ctx = r.Context()
		}
		state, _ := ctx.Value(requestLogKey{}).(*requestLog)
		res := NewProblem(ctx, err)
		if state == nil {
			logger.LogAttrs(ctx, statusLevel(res.StatusCode), "http_request_error", slog.Int("status", res.StatusCode), slog.String("error", err.Error()))
		} else if state.written {
			return
		}
		// Within the request logger the writer records its own failures.
		if writeErr := writeProblem(w, res); writeErr != nil && state == nil {
			logger.ErrorContext(ctx, "failed to write error response", "error", writeErr)
		}
	}
}

// NotFound answers unknown API routes with a problem instead of ogen's plain text.
func NotFound(w http.ResponseWriter, r *http.Request) {
	_ = writeProblem(w, NewProblem(r.Context(), pkg.Wrap(pkg.ErrNotFound, nil, "no such API route")))
}

func writeProblem(w http.ResponseWriter, res *corev1.ProblemStatusCodeWithHeaders) error {
	w.Header().Set("Content-Type", "application/problem+json")
	if res.RetryAfter.IsSet() {
		w.Header().Set("Retry-After", strconv.Itoa(res.RetryAfter.Value))
	}
	w.WriteHeader(res.StatusCode)
	if res.StatusCode == statusClientClosedRequest {
		return nil
	}
	body, err := res.Response.MarshalJSON()
	if err != nil {
		return fmt.Errorf("encode problem: %w", err)
	}
	if _, err := w.Write(body); err != nil { //nolint:gosec // A JSON body with the problem+json content type, not HTML.
		return fmt.Errorf("write problem: %w", err)
	}
	return nil
}
