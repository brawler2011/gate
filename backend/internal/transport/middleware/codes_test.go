package middleware

import (
	"slices"
	"testing"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/stretchr/testify/require"
)

func TestSpecListsEveryErrorCode(t *testing.T) {
	var spec, code []string
	for _, c := range corev1.ErrorCode("").AllValues() {
		spec = append(spec, string(c))
	}
	for _, c := range pkg.Codes() {
		code = append(code, string(c))
	}
	slices.Sort(spec)
	slices.Sort(code)
	require.Equal(t, spec, code, "contracts/core/v1/openapi.yaml ErrorCode must list exactly pkg.Codes()")
}
