package pkg

import (
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
	Pointer string `json:"pointer"`
	Message string `json:"message"`
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
