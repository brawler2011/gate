package middleware

import (
	"reflect"
	"testing"

	corev1 "github.com/brawler2011/contracts/core/v1"
)

func TestStrictAuthzPoliciesCoverStrictOperations(t *testing.T) {
	handlerInterfaceType := reflect.TypeOf((*corev1.Handler)(nil)).Elem()

	for i := 0; i < handlerInterfaceType.NumMethod(); i++ {
		operationID := handlerInterfaceType.Method(i).Name
		evaluators, ok := endpointPolicies[operationID]
		if !ok || len(evaluators) == 0 {
			t.Errorf("missing authz policy for operation %q", operationID)
		}
	}
}

func TestStrictAuthzPoliciesContainOnlyStrictOperations(t *testing.T) {
	handlerInterfaceType := reflect.TypeOf((*corev1.Handler)(nil)).Elem()

	for operationID := range endpointPolicies {
		if _, ok := handlerInterfaceType.MethodByName(operationID); !ok {
			t.Errorf("unknown operation in authz policy map %q", operationID)
		}
	}
}
