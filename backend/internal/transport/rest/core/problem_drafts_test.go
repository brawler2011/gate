package core

import (
	"reflect"
	"testing"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/stretchr/testify/require"
)

// fill sets every field reachable from v to a non-zero value, so that a
// field added to the draft model later is covered without editing this test.
func fill(v reflect.Value, n *int) {
	*n++
	switch v.Kind() {
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		fill(v.Elem(), n)
	case reflect.Struct:
		for i := range v.NumField() {
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

// A draft field the contract does not carry would be lost on the next save
// without any error, because the workshop saves what it read.
func TestDraftSurvivesContractRoundTrip(t *testing.T) {
	var d model.Draft
	n := 0
	fill(reflect.ValueOf(&d).Elem(), &n)
	d.Type = "scoring"
	d.Subtasks[0].Policy = "each"
	d.Tests[0].Input.Mode = "generator"
	d.Tests[0].Answer.Mode = "generated"

	contract, err := draftContract[corev1.ProblemDraft](d)
	require.NoError(t, err)
	back, err := draftModel(contract)
	require.NoError(t, err)
	require.Equal(t, d, *back)
}
