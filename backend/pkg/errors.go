package pkg

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

// Code is the stable machine-readable error code clients branch and translate on.
type Code string

const (
	CodeInvalid         Code = "invalid"
	CodeUnauthenticated Code = "unauthenticated"
	CodeForbidden       Code = "forbidden"
	CodeNotFound        Code = "not_found"
	CodeConflict        Code = "conflict"
	CodePayloadTooLarge Code = "payload_too_large"
	CodeRateLimited     Code = "rate_limited"
	CodeInternal        Code = "internal"
	CodeNotImplemented  Code = "not_implemented"
)

var statuses = map[Code]int{
	CodeInvalid:         http.StatusBadRequest,
	CodeUnauthenticated: http.StatusUnauthorized,
	CodeForbidden:       http.StatusForbidden,
	CodeNotFound:        http.StatusNotFound,
	CodeConflict:        http.StatusConflict,
	CodePayloadTooLarge: http.StatusRequestEntityTooLarge,
	CodeRateLimited:     http.StatusTooManyRequests,
	CodeInternal:        http.StatusInternalServerError,
	CodeNotImplemented:  http.StatusNotImplemented,
}

// kinds maps each specific code to the generic code that gives its status.
var kinds = map[Code]Code{}

// Error is an error the client may see: Message is public English text, Err is the private cause.
type Error struct {
	Code    Code
	Message string
	Params  map[string]any
	Err     error
}

// Sentinels: errors.Is(err, ErrConflict) matches every error whose code is or specializes conflict.
var (
	ErrInvalid         = &Error{Code: CodeInvalid}
	ErrUnauthenticated = &Error{Code: CodeUnauthenticated}
	ErrForbidden       = &Error{Code: CodeForbidden}
	ErrNotFound        = &Error{Code: CodeNotFound}
	ErrConflict        = &Error{Code: CodeConflict}
	ErrPayloadTooLarge = &Error{Code: CodePayloadTooLarge}
	ErrInternal        = &Error{Code: CodeInternal}
)

// Specific codes; each must also be listed in the OpenAPI ErrorCode enum.
var (
	// A serializable transaction lost a race; the caller may retry with a fresh precheck.
	ErrConcurrentUpdate       = NewCode("concurrent_update", CodeConflict)
	ErrInUse                  = NewCode("in_use", CodeConflict)
	ErrAlreadyMember          = NewCode("already_member", CodeConflict)
	ErrRequestPending         = NewCode("request_pending", CodeConflict)
	ErrUsernameTaken          = NewCode("username_taken", CodeConflict)
	ErrEmailTaken             = NewCode("email_taken", CodeConflict)
	ErrOrganizationLoginTaken = NewCode("organization_login_taken", CodeConflict)
	ErrContestLoginTaken      = NewCode("contest_login_taken", CodeConflict)
	ErrTeamSlugTaken          = NewCode("team_slug_taken", CodeConflict)
	ErrProblemShortNameTaken  = NewCode("problem_short_name_taken", CodeConflict)
	ErrProblemInContest       = NewCode("problem_in_contest", CodeConflict)
	ErrEmailNotVerified       = NewCode("email_not_verified", CodeForbidden)
	ErrWrongPassword          = NewCode("wrong_password", CodeInvalid)
	ErrInvitationPending      = NewCode("invitation_pending", CodeConflict)
	ErrRequestResolved        = NewCode("request_resolved", CodeConflict)
	ErrInviteOnly             = NewCode("invite_only", CodeForbidden)
	ErrNotOrgMember           = NewCode("not_org_member", CodeInvalid)

	// Field validation; params carry the violated bounds (min, max).
	ErrRequired       = NewCode("required", CodeInvalid)
	ErrLength         = NewCode("length", CodeInvalid)
	ErrFormat         = NewCode("format", CodeInvalid)
	ErrReserved       = NewCode("reserved", CodeInvalid)
	ErrRange          = NewCode("range", CodeInvalid)
	ErrEndBeforeStart = NewCode("end_before_start", CodeInvalid)
)

// NewCode registers a specific code whose status is that of kind.
func NewCode(code, kind Code) *Error {
	if _, ok := statuses[kind]; !ok {
		panic("unknown error kind " + string(kind))
	}
	kinds[code] = kind
	return &Error{Code: code}
}

func (e *Error) Error() string {
	parts := []string{string(e.Code)}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	if e.Err != nil {
		parts = append(parts, e.Err.Error())
	}
	return strings.Join(parts, ": ")
}

func (e *Error) Unwrap() error { return e.Err }

func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok || t.Message != "" || t.Err != nil || t.Params != nil {
		return false
	}
	return e.Code == t.Code || kinds[e.Code] == t.Code
}

// Wrap returns an error with code's code, a public message and a private cause.
func Wrap(code *Error, cause error, message string) error {
	return &Error{Code: code.Code, Message: message, Err: cause}
}

// OnNotFound gives a missing entity the caller's meaning and passes every other failure through.
func OnNotFound(err error, code *Error, message string) error {
	if errors.Is(err, ErrNotFound) {
		return Wrap(code, err, message)
	}
	return err
}

// WithCause adds why ctx was cancelled to an error the cancellation caused.
func WithCause(ctx context.Context, err error) error {
	if err == nil || ctx.Err() == nil {
		return err
	}
	cause := context.Cause(ctx)
	if cause == nil || errors.Is(err, cause) {
		return err
	}
	return errors.Join(err, cause)
}

// Codes lists every code the API can return.
func Codes() []Code {
	codes := make([]Code, 0, len(statuses)+len(kinds))
	for code := range statuses {
		codes = append(codes, code)
	}
	for code := range kinds {
		codes = append(codes, code)
	}
	return codes
}

// CodeOf returns the code of the outermost Error in err's chain, internal when there is none.
func CodeOf(err error) Code {
	var e *Error
	var rateErr *RateLimitError
	var sizeErr *http.MaxBytesError
	switch {
	case errors.As(err, &e):
		return e.Code
	case errors.As(err, &rateErr):
		return CodeRateLimited
	case errors.As(err, &sizeErr):
		return CodePayloadTooLarge
	}
	return CodeInternal
}

func HTTPStatus(code Code) int {
	if kind, ok := kinds[code]; ok {
		code = kind
	}
	return statuses[code]
}

// Pointer is an RFC 6901 JSON Pointer into the request body.
type FieldError struct {
	Pointer string
	// Empty means CodeInvalid.
	Code    Code
	Params  map[string]any
	Message string
}

type FieldErrors []FieldError

func (e FieldErrors) Error() string {
	parts := make([]string, len(e))
	for i, f := range e {
		parts[i] = f.Pointer + ": " + f.Message
	}
	return strings.Join(parts, "; ")
}

func (e FieldErrors) Unwrap() error { return ErrInvalid }

// Invalid is a validation failure of one value; AtField places it in the request.
func Invalid(code *Error, message string, params map[string]any) error {
	return &Error{Code: code.Code, Message: message, Params: params}
}

// AtField reports err as a failure of the value at pointer; errors without a code become invalid.
func AtField(pointer string, err error) error {
	var fields FieldErrors
	if err == nil || errors.As(err, &fields) {
		return err
	}
	f := FieldError{Pointer: pointer, Code: CodeInvalid, Message: err.Error()}
	var e *Error
	if errors.As(err, &e) {
		f.Code, f.Params, f.Message = e.Code, e.Params, e.Message
	}
	return FieldErrors{f}
}

// Fields merges the field errors of errs into one error, nil when there are none.
func Fields(errs ...error) error {
	var out FieldErrors
	for _, err := range errs {
		if err == nil {
			continue
		}
		var fields FieldErrors
		if !errors.As(err, &fields) {
			fields = FieldErrors{{Code: CodeInvalid, Message: err.Error()}}
		}
		out = append(out, fields...)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
