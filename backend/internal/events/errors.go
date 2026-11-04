package events

import "errors"

type PermanentError struct{ Cause error }

func (e *PermanentError) Error() string { return e.Cause.Error() }
func (e *PermanentError) Unwrap() error { return e.Cause }
func IsPermanent(err error) bool {
	var permanent *PermanentError
	return errors.As(err, &permanent)
}
