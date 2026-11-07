package page

import (
	"math"
	"testing"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	for _, tc := range []struct {
		name       string
		page, size int32
		err        error
	}{
		{name: "minimum", page: 1, size: 1},
		{name: "maximum size", page: 2, size: 100},
		{name: "large page", page: math.MaxInt32, size: 50},
		{name: "zero page", size: 50, err: pkg.ErrBadInput},
		{name: "zero size", page: 1, err: pkg.ErrBadInput},
		{name: "negative page", page: -1, size: 50, err: pkg.ErrBadInput},
		{name: "negative size", page: 1, size: -1, err: pkg.ErrBadInput},
		{name: "oversized", page: 1, size: 101, err: pkg.ErrBadInput},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorIs(t, Validate(tc.page, tc.size), tc.err)
		})
	}
}

func TestNormalize(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		page, size, defaultSize int32
		wantPage, wantSize      int32
		err                     error
	}{
		{name: "contest defaults", defaultSize: 50, wantPage: 1, wantSize: 50},
		{name: "draft defaults", defaultSize: 20, wantPage: 1, wantSize: 20},
		{name: "only page default", size: 100, defaultSize: 50, wantPage: 1, wantSize: 100},
		{name: "only size default", page: 3, defaultSize: 20, wantPage: 3, wantSize: 20},
		{name: "minimum", page: 1, size: 1, defaultSize: 50, wantPage: 1, wantSize: 1},
		{name: "maximum size", page: 2, size: 100, defaultSize: 50, wantPage: 2, wantSize: 100},
		{name: "negative page", page: -1, size: 50, defaultSize: 50, err: pkg.ErrBadInput},
		{name: "negative size", page: 1, size: -1, defaultSize: 50, err: pkg.ErrBadInput},
		{name: "oversized", page: 1, size: 101, defaultSize: 50, err: pkg.ErrBadInput},
		{name: "zero default", err: pkg.ErrBadInput},
		{name: "negative default", defaultSize: -1, err: pkg.ErrBadInput},
		{name: "oversized default", defaultSize: 101, err: pkg.ErrBadInput},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page, size, err := Normalize(tc.page, tc.size, tc.defaultSize)
			require.ErrorIs(t, err, tc.err)
			if tc.err == nil {
				require.Equal(t, tc.wantPage, page)
				require.Equal(t, tc.wantSize, size)
			}
		})
	}
}
