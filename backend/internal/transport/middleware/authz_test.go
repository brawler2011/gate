package middleware

import (
	"reflect"
	"testing"

	corev1 "github.com/gate149/contracts/core/v1"
)

func TestStrictAuthzPoliciesCoverStrictOperations(t *testing.T) {
	strictInterfaceType := reflect.TypeOf((*corev1.StrictServerInterface)(nil)).Elem()

	for i := 0; i < strictInterfaceType.NumMethod(); i++ {
		operationID := strictInterfaceType.Method(i).Name
		if _, ok := strictAuthzPolicies[operationID]; !ok {
			t.Errorf("missing authz policy for operation %q", operationID)
		}
	}
}

func TestStrictAuthzPoliciesContainOnlyStrictOperations(t *testing.T) {
	strictInterfaceType := reflect.TypeOf((*corev1.StrictServerInterface)(nil)).Elem()

	for operationID := range strictAuthzPolicies {
		if _, ok := strictInterfaceType.MethodByName(operationID); !ok {
			t.Errorf("unknown operation in authz policy map %q", operationID)
		}
	}
}
