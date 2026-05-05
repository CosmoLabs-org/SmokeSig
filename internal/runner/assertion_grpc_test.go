//go:build grpc

package runner

import (
	"net"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func startTestGRPCServer(t *testing.T) (addr string, healthSrv *health.Server, stop func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	srv := grpc.NewServer()
	healthSrv = health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	go func() { _ = srv.Serve(lis) }()
	return lis.Addr().String(), healthSrv, func() { srv.Stop() }
}

func TestCheckGRPCHealth_OverallServing(t *testing.T) {
	addr, healthSrv, stop := startTestGRPCServer(t)
	defer stop()
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	result := CheckGRPCHealth(&schema.GRPCHealthCheck{
		Address: addr,
		Service: "",
	})
	if !result.Passed {
		t.Errorf("expected pass, got actual=%q", result.Actual)
	}
	if result.Actual != "SERVING" {
		t.Errorf("expected Actual=SERVING, got %q", result.Actual)
	}
}

func TestCheckGRPCHealth_SpecificServiceServing(t *testing.T) {
	addr, healthSrv, stop := startTestGRPCServer(t)
	defer stop()
	healthSrv.SetServingStatus("my.Service", healthpb.HealthCheckResponse_SERVING)

	result := CheckGRPCHealth(&schema.GRPCHealthCheck{
		Address: addr,
		Service: "my.Service",
	})
	if !result.Passed {
		t.Errorf("expected pass, got actual=%q", result.Actual)
	}
}

func TestCheckGRPCHealth_SpecificServiceNotServing(t *testing.T) {
	addr, healthSrv, stop := startTestGRPCServer(t)
	defer stop()
	healthSrv.SetServingStatus("bad.Service", healthpb.HealthCheckResponse_NOT_SERVING)

	result := CheckGRPCHealth(&schema.GRPCHealthCheck{
		Address: addr,
		Service: "bad.Service",
	})
	if result.Passed {
		t.Errorf("expected fail, service is NOT_SERVING")
	}
	if result.Actual != "NOT_SERVING" {
		t.Errorf("expected Actual=NOT_SERVING, got %q", result.Actual)
	}
}

func TestCheckGRPCHealth_DialFailure(t *testing.T) {
	result := CheckGRPCHealth(&schema.GRPCHealthCheck{
		Address: "127.0.0.1:1",
		Timeout: schema.Duration{Duration: 500 * time.Millisecond},
	})
	if result.Passed {
		t.Errorf("expected fail for non-existent address")
	}
}
