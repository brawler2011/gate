package core

import (
	"reflect"
	"testing"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/stretchr/testify/require"
)

// Fill every reachable field so model additions are covered automatically; recursive types nest once.
func fill(v reflect.Value, n *int) {
	*n++
	switch v.Kind() {
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		fill(v.Elem(), n)
	case reflect.Struct:
		for i := range v.NumField() {
			if f := v.Field(i); f.Kind() == reflect.Slice && f.Type().Elem() == v.Type() {
				f.Set(reflect.MakeSlice(f.Type(), 0, 0))
				continue
			}
			fill(v.Field(i), n)
		}
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		fill(v.Index(0), n)
	case reflect.Array:
		for i := range v.Len() {
			v.Index(i).SetUint(uint64(*n + i)) //nolint:gosec // small test values
		}
	case reflect.String:
		v.SetString("text")
	case reflect.Int, reflect.Int32, reflect.Int64:
		v.SetInt(int64(*n * 100))
	case reflect.Bool:
		v.SetBool(true)
	default:
		panic("fill: unsupported kind " + v.Kind().String())
	}
}

// Every draft field must round-trip through the contract to survive a workshop save.
func TestDraftSurvivesContractRoundTrip(t *testing.T) {
	var d model.Draft
	n := 0
	fill(reflect.ValueOf(&d).Elem(), &n)
	d.Type, d.Grading = "standard", "points"
	d.Testset.Aggregate = "min"
	d.Tests[0].Input.Mode = "generator"
	d.Tests[0].Answer.Mode = "generated"

	contract, err := draftContract[corev1.ProblemDraft](d)
	require.NoError(t, err)
	back, err := draftModel(contract)
	require.NoError(t, err)
	require.Equal(t, d, *back)
}
