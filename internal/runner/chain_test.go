package runner

import (
	"testing"

	"github.com/CosmoLabs-org/cosmo-smoke/internal/schema"
)

func TestChainIntegration_ExtractAndResolve(t *testing.T) {
	cfg := &schema.SmokeConfig{
		Version: 1,
		Project: "chain-test",
		Tests: []schema.Test{
			{
				Name: "extract-token",
				Run:  `echo '{"token":"my-jwt-123","user":"gab"}'`,
				Expect: schema.Expect{
					ExitCode: intPtr(0),
					JSONField: &schema.JSONFieldCheck{
						Path:    "token",
						Extract: "jwt_token",
					},
				},
			},
			{
				Name: "use-token",
				Run:  `echo "Bearer {{ .Vars.jwt_token }}"`,
				Expect: schema.Expect{
					ExitCode:       intPtr(0),
					StdoutContains: "Bearer my-jwt-123",
				},
			},
		},
	}

	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	suite, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if suite.Failed > 0 {
		t.Errorf("expected 0 failures, got %d", suite.Failed)
		for _, tr := range suite.Tests {
			if !tr.Passed && !tr.AllowedFailure {
				for _, a := range tr.Assertions {
					if !a.Passed {
						t.Logf("  FAIL %s: %s = %s (want %s)", tr.Name, a.Type, a.Actual, a.Expected)
					}
				}
			}
		}
	}

	// Verify token was stored and is masked
	if r.Vars == nil {
		t.Fatal("Vars should be initialized")
	}
	val, ok := r.Vars.Get("jwt_token")
	if !ok {
		t.Fatal("jwt_token should be extracted")
	}
	if val != "my-jwt-123" {
		t.Errorf("jwt_token = %q, want %q", val, "my-jwt-123")
	}
}

func TestChainIntegration_ExtractFromStdoutMatches(t *testing.T) {
	cfg := &schema.SmokeConfig{
		Version: 1,
		Project: "chain-test",
		Tests: []schema.Test{
			{
				Name: "start-server",
				Run:  `echo "Server started port=9090 ok"`,
				Expect: schema.Expect{
					ExitCode:      intPtr(0),
					StdoutMatches: `port=(\d+)`,
					Extract:       "server_port",
				},
			},
			{
				Name: "use-port",
				Run:  `echo "http://localhost:{{ .Vars.server_port }}/health"`,
				Expect: schema.Expect{
					ExitCode:       intPtr(0),
					StdoutContains: "http://localhost:9090/health",
				},
			},
		},
	}

	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	suite, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if suite.Failed > 0 {
		t.Errorf("expected 0 failures, got %d", suite.Failed)
		for _, tr := range suite.Tests {
			if !tr.Passed && !tr.AllowedFailure {
				for _, a := range tr.Assertions {
					if !a.Passed {
						t.Logf("  FAIL %s: %s = %s (want %s)", tr.Name, a.Type, a.Actual, a.Expected)
					}
				}
			}
		}
	}
}

func TestChainIntegration_SecretMasking(t *testing.T) {
	store := NewVarStore()
	store.Set("api_token", "super-secret-value")
	store.Set("host", "localhost")

	masked := store.Mask("token=super-secret-value host=localhost")
	if masked != "token=***REDACTED*** host=localhost" {
		t.Errorf("got %q", masked)
	}
}

func TestChainIntegration_FailedTestNoExtract(t *testing.T) {
	cfg := &schema.SmokeConfig{
		Version: 1,
		Project: "chain-test",
		Tests: []schema.Test{
			{
				Name: "failing-extract",
				Run:  `echo '{"token":"should-not-extract"}' && exit 1`,
				Expect: schema.Expect{
					ExitCode: intPtr(0), // wrong — test will fail
					JSONField: &schema.JSONFieldCheck{
						Path:    "token",
						Extract: "jwt_token",
					},
				},
			},
		},
	}

	r := &Runner{Config: cfg, Reporter: &noopReporter{}, ConfigDir: t.TempDir()}
	suite, err := r.Run(RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if suite.Passed != 0 {
		t.Errorf("expected 0 passes, got %d", suite.Passed)
	}
	// Verify token was NOT stored (test failed, so no extract)
	_, ok := r.Vars.Get("jwt_token")
	if ok {
		t.Error("jwt_token should NOT be extracted from a failing test")
	}
}
