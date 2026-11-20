package pkg_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/stretchr/testify/require"
)

var errTaken = pkg.NewCode("test_taken", pkg.CodeConflict)

func TestErrorMatchesItsCodeAndKind(t *testing.T) {
	err := fmt.Errorf("create: %w", pkg.Wrap(errTaken, errors.New("unique violation"), "login is taken"))
	require.ErrorIs(t, err, errTaken)
	require.ErrorIs(t, err, pkg.ErrConflict)
	require.NotErrorIs(t, err, pkg.ErrInvalid)
	require.NotErrorIs(t, pkg.ErrConflict, errTaken, "a generic error is not a specific one")
	require.NotErrorIs(t, pkg.ErrConflict, pkg.Wrap(pkg.ErrConflict, nil, "x"), "only bare sentinels are match targets")
	require.Equal(t, errTaken.Code, pkg.CodeOf(err))
	require.Equal(t, http.StatusConflict, pkg.HTTPStatus(pkg.CodeOf(err)))
	require.Equal(t, pkg.CodeInternal, pkg.CodeOf(errors.New("plain")))
	require.Equal(t, pkg.CodeInvalid, pkg.CodeOf(pkg.FieldErrors{{Pointer: "/a", Message: "bad"}}))
}
