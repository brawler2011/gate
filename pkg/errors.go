package pkg

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
)

var (
	NoPermission       = errors.New("no permission")
	ErrUnauthenticated = errors.New("unauthenticated")
	ErrUnhandled       = errors.New("unhandled")
	ErrNotFound        = errors.New("not found")
	ErrBadInput        = errors.New("bad input")
	ErrInternal        = errors.New("internal")
)

type CustomError struct {
	Basic   error
	Cause   error
	Message string
	Op      string
}

func (e *CustomError) Error() string {
	var parts []string
	if e.Op != "" {
		parts = append(parts, e.Op)
	}
	if e.Basic != nil {
		parts = append(parts, e.Basic.Error())
	}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	if e.Cause != nil {
		parts = append(parts, e.Cause.Error())
	}
	return strings.Join(parts, ": ")
}

func (e *CustomError) Unwrap() []error {
	return []error{e.Basic, e.Cause}
}

func getCallerInfo() string {
	pc, _, line, ok := runtime.Caller(2)
	if !ok {
		return ""
	}
	fn := runtime.FuncForPC(pc)
	return fmt.Sprintf("%s:%d", fn.Name(), line)
}

func Wrap(basic error, err error, msg string) error {
	return &CustomError{
		Basic:   basic,
		Cause:   err,
		Message: msg,
		Op:      getCallerInfo(),
	}
}

func ToREST(err error) int {
	switch {
	case errors.Is(err, ErrUnauthenticated):
		return http.StatusUnauthorized
	case errors.Is(err, ErrBadInput):
		return http.StatusBadRequest
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrInternal):
		return http.StatusInternalServerError
	case errors.Is(err, NoPermission):
		return http.StatusForbidden
	}

	return http.StatusInternalServerError
}

func HandleContextErr(err error) error {
	if errors.Is(err, context.Canceled) {
		return Wrap(ErrInternal, err, "context canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Wrap(ErrInternal, err, "context deadline exceeded")
	}
	return nil
}
