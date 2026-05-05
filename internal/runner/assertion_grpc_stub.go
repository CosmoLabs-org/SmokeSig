//go:build !grpc

package runner

import (
	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// CheckGRPCHealth returns an error when built without the grpc tag.
func CheckGRPCHealth(check *schema.GRPCHealthCheck) AssertionResult {
	return AssertionResult{
		Type:     "grpc_health",
		Expected: check.Address,
		Actual:   "grpc_health not available — rebuild with -tags grpc",
		Passed:   false,
	}
}

// CheckGRPCHealthWithTrace is the stub version (delegates to stub).
func CheckGRPCHealthWithTrace(check *schema.GRPCHealthCheck, span *SpanContext) AssertionResult {
	return CheckGRPCHealth(check)
}
