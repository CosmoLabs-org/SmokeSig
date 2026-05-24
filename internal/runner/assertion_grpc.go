//go:build grpc

package runner

import (
	"context"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// CheckGRPCHealth queries grpc.health.v1.Health/Check and passes if SERVING.
func CheckGRPCHealth(check *schema.GRPCHealthCheck) AssertionResult {
	timeout := check.Timeout.Duration
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if len(check.Metadata) > 0 {
		md := metadata.New(check.Metadata)
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	var creds credentials.TransportCredentials
	if check.UseTLS {
		creds = credentials.NewTLS(nil)
	} else {
		creds = insecure.NewCredentials()
	}

	conn, err := grpc.NewClient(check.Address, grpc.WithTransportCredentials(creds))
	if err != nil {
		return AssertionResult{
			Type:     "grpc_health",
			Expected: check.Address,
			Actual:   "dial error: " + err.Error(),
			Passed:   false,
		}
	}
	defer conn.Close()

	client := healthpb.NewHealthClient(conn)
	resp, err := client.Check(ctx, &healthpb.HealthCheckRequest{Service: check.Service})
	if err != nil {
		return AssertionResult{
			Type:     "grpc_health",
			Expected: "SERVING",
			Actual:   "rpc error: " + err.Error(),
			Passed:   false,
		}
	}

	status := resp.GetStatus().String()
	return AssertionResult{
		Type:     "grpc_health",
		Expected: "SERVING",
		Actual:   status,
		Passed:   status == "SERVING",
	}
}

// CheckGRPCHealthWithTrace is like CheckGRPCHealth but injects traceparent into metadata.
func CheckGRPCHealthWithTrace(check *schema.GRPCHealthCheck, span *SpanContext) AssertionResult {
	clone := *check
	clone.Metadata = make(map[string]string, len(check.Metadata)+1)
	for k, v := range check.Metadata {
		clone.Metadata[k] = v
	}
	clone.Metadata["traceparent"] = span.Traceparent()
	return CheckGRPCHealth(&clone)
}
