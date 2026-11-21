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
	"github.com/go-faster/jx"
	"github.com/ogen-go/ogen/ogenerrors"
	"github.com/ogen-go/ogen/validate"
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
	var fields pkg.FieldErrors
	switch {
	case errors.Is(err, context.Canceled) && ctx.Err() != nil:
		code, status = pkg.CodeInternal, statusClientClosedRequest
	case errors.As(err, &fields):
		detail = fields.Error()
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
			fields = schemaFields(err)
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
	if status < 500 {
		for _, f := range fields {
			code := f.Code
			if code == "" {
				code = pkg.CodeInvalid
			}
			res.Response.Errors = append(res.Response.Errors, corev1.FieldProblem{
				Pointer: f.Pointer,
				Code:    corev1.ErrorCode(code),
				Params:  fieldParams(f.Params),
				Detail:  corev1.NewOptString(f.Message),
			})
		}
	}
	return res
}

// schemaFields turns ogen's schema validation failures into field errors.
func schemaFields(err error) pkg.FieldErrors {
	var verr *validate.Error
	if !errors.As(err, &verr) {
		return nil
	}
	out := make(pkg.FieldErrors, 0, len(verr.Fields))
	for _, f := range verr.Fields {
		field := pkg.FieldError{Pointer: "/" + f.Name, Code: pkg.CodeInvalid, Message: f.Error.Error()}
		var minErr *validate.MinLengthError
		var maxErr *validate.MaxLengthError
		var regexErr *validate.NoRegexMatchError
		switch {
		case errors.Is(f.Error, validate.ErrFieldRequired):
			field.Code = pkg.ErrRequired.Code
		case errors.As(f.Error, &minErr):
			field.Code, field.Params = pkg.ErrLength.Code, map[string]any{"min": minErr.MinLength}
		case errors.As(f.Error, &maxErr):
			field.Code, field.Params = pkg.ErrLength.Code, map[string]any{"max": maxErr.MaxLength}
		case errors.As(f.Error, &regexErr):
			field.Code = pkg.ErrFormat.Code
		}
		out = append(out, field)
	}
	return out
}

func rawParams(params map[string]any) map[string]jx.Raw {
	out := make(map[string]jx.Raw, len(params))
	for k, v := range params {
		raw, err := json.Marshal(v)
		if err != nil {
			continue
		}
		out[k] = raw
	}
	return out
}

func problemParams(params map[string]any) corev1.OptProblemParams {
	if len(params) == 0 {
		return corev1.OptProblemParams{}
	}
	return corev1.NewOptProblemParams(rawParams(params))
}

func fieldParams(params map[string]any) corev1.OptFieldProblemParams {
	if len(params) == 0 {
		return corev1.OptFieldProblemParams{}
	}
	return corev1.NewOptFieldProblemParams(rawParams(params))
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
