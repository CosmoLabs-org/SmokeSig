//go:build !grpc

package runner

import (
	"strings"
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

func TestCheckGRPCHealth_StubReturns(t *testing.T) {
	result := CheckGRPCHealth(&schema.GRPCHealthCheck{Address: "localhost:9090"})
	if result.Passed {
		t.Error("stub should not pass")
	}
	if !strings.Contains(result.Actual, "grpc") {
		t.Error("should mention grpc in output, got:", result.Actual)
	}
}
