package schema

import (
	"strings"
	"testing"
)

func TestValidate_ValidConfig(t *testing.T) {
	exitCode := 0
	cfg := &SmokeConfig{
		Version: 1,
		Project: "myapp",
		Tests: []Test{
			{Name: "test1", Run: "echo hi", Expect: Expect{ExitCode: &exitCode}},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_MissingProject(t *testing.T) {
	exitCode := 0
	cfg := &SmokeConfig{
		Version: 1,
		Tests: []Test{
			{Name: "test1", Run: "echo hi", Expect: Expect{ExitCode: &exitCode}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "project name is required") {
		t.Errorf("error = %q, want mention of project", err.Error())
	}
}

func TestValidate_InvalidVersion(t *testing.T) {
	exitCode := 0
	cfg := &SmokeConfig{
		Version: 2,
		Project: "myapp",
		Tests: []Test{
			{Name: "test1", Run: "echo hi", Expect: Expect{ExitCode: &exitCode}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unsupported version") {
		t.Errorf("error = %q, want mention of version", err.Error())
	}
}

func TestValidate_NoTests(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "myapp",
		Tests:   []Test{},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "at least one test") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestValidate_TestMissingName(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "myapp",
		Tests: []Test{
			{Run: "echo hi"},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestValidate_TestMissingRun(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "myapp",
		Tests: []Test{
			{Name: "test1"},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "run command is required") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 0,
		Tests:   []Test{},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	// Should have: bad version, missing project, no tests
	if len(ve.Errors) < 3 {
		t.Errorf("expected at least 3 errors, got %d: %v", len(ve.Errors), ve.Errors)
	}
}

func TestValidate_RetryCountZero(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "myapp",
		Tests: []Test{
			{
				Name: "test1", Run: "echo hi",
				Retry: &RetryPolicy{Count: 0, Backoff: Duration{Duration: 1e9}},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for retry.count = 0")
	}
	if !strings.Contains(err.Error(), "retry.count must be >= 1") {
		t.Errorf("error = %q, want mention of retry.count", err.Error())
	}
}

func TestValidate_RetryBackoffZero(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "myapp",
		Tests: []Test{
			{
				Name: "test1", Run: "echo hi",
				Retry: &RetryPolicy{Count: 3, Backoff: Duration{Duration: 0}},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for retry.backoff = 0")
	}
	if !strings.Contains(err.Error(), "retry.backoff must be > 0") {
		t.Errorf("error = %q, want mention of retry.backoff", err.Error())
	}
}

func TestValidate_RetryValid(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "myapp",
		Tests: []Test{
			{
				Name: "test1", Run: "echo hi",
				Retry: &RetryPolicy{Count: 3, Backoff: Duration{Duration: 1e9}},
			},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("unexpected error for valid retry block: %v", err)
	}
}

func TestValidate_DockerContainerRunning_MissingName(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "t1", Run: "true", Expect: Expect{DockerContainer: &DockerContainerCheck{Name: ""}}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Error("expected validation error for empty docker_container_running.name")
	}
}

func TestValidate_DockerImageExists_MissingImage(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{
			{Name: "t1", Run: "true", Expect: Expect{DockerImage: &DockerImageCheck{Image: ""}}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Error("expected validation error for empty docker_image_exists.image")
	}
}

func TestValidate_OTelTraceRequiresJaegerURL(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		Tests: []Test{{
			Name: "otel",
			Expect: Expect{
				OTelTrace: &OTelTraceCheck{MinSpans: 1},
			},
		}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for otel_trace without jaeger_url")
	}
	if !strings.Contains(err.Error(), "otel_trace.jaeger_url") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_OTelEnabledRequiresJaegerURL(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "test",
		OTel:    OTelConfig{Enabled: true},
		Tests:   []Test{{Name: "t", Run: "true"}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for otel enabled without jaeger_url")
	}
	if !strings.Contains(err.Error(), "otel.jaeger_url") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_RetryOnTraceOnly_WithoutOTelTrace(t *testing.T) {
	exitCode := 0
	cfg := &SmokeConfig{
		Version: 1,
		Project: "myapp",
		Tests: []Test{
			{
				Name: "test1", Run: "echo hi",
				Expect: Expect{ExitCode: &exitCode},
				Retry: &RetryPolicy{Count: 3, Backoff: Duration{Duration: 1e9}, RetryOnTraceOnly: true},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for retry_on_trace_only without otel_trace")
	}
	if !strings.Contains(err.Error(), "retry_on_trace_only requires otel_trace") {
		t.Errorf("error = %q, want mention of retry_on_trace_only", err.Error())
	}
}

func TestValidate_RetryOnTraceOnly_WithOTelTrace(t *testing.T) {
	exitCode := 0
	cfg := &SmokeConfig{
		Version: 1,
		Project: "myapp",
		Tests: []Test{
			{
				Name: "test1", Run: "echo hi",
				Expect: Expect{
					ExitCode:  &exitCode,
					OTelTrace: &OTelTraceCheck{JaegerURL: "http://localhost:16686"},
				},
				Retry: &RetryPolicy{Count: 3, Backoff: Duration{Duration: 1e9}, RetryOnTraceOnly: true},
			},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("unexpected error for retry_on_trace_only with otel_trace: %v", err)
	}
}

func TestValidate_OTelTrace_InvalidBackend(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "myapp",
		Tests: []Test{
			{
				Name: "test1", Run: "echo hi",
				Expect: Expect{
					OTelTrace: &OTelTraceCheck{JaegerURL: "http://localhost:16686", Backend: "zipkin"},
				},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid backend")
	}
	if !strings.Contains(err.Error(), "otel_trace.backend must be") {
		t.Errorf("error = %q, want mention of valid backends", err.Error())
	}
}

func TestValidate_OTelTrace_HoneycombRequiresAPIKey(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "myapp",
		Tests: []Test{
			{
				Name: "test1", Run: "echo hi",
				Expect: Expect{
					OTelTrace: &OTelTraceCheck{JaegerURL: "https://api.honeycomb.io", Backend: "honeycomb"},
				},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for honeycomb without api_key")
	}
	if !strings.Contains(err.Error(), "api_key is required for honeycomb") {
		t.Errorf("error = %q, want mention of api_key", err.Error())
	}
}

func TestValidate_OTelTrace_DatadogRequiresAPIKey(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "myapp",
		Tests: []Test{
			{
				Name: "test1", Run: "echo hi",
				Expect: Expect{
					OTelTrace: &OTelTraceCheck{JaegerURL: "https://api.datadoghq.com", Backend: "datadog"},
				},
			},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for datadog without api_key")
	}
	if !strings.Contains(err.Error(), "api_key is required for datadog") {
		t.Errorf("error = %q, want mention of api_key", err.Error())
	}
}

func TestValidate_OTelTrace_TempoValidWithJaegerURL(t *testing.T) {
	cfg := &SmokeConfig{
		Version: 1,
		Project: "myapp",
		Tests: []Test{
			{
				Name: "test1", Run: "echo hi",
				Expect: Expect{
					OTelTrace: &OTelTraceCheck{JaegerURL: "http://tempo:3200", Backend: "tempo"},
				},
			},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("unexpected error for valid tempo config: %v", err)
	}
}

func TestValidate_FileSize(t *testing.T) {
	t.Run("valid file_size with both thresholds", func(t *testing.T) {
		min := int64(100)
		max := int64(5000)
		cfg := &SmokeConfig{
			Version: 1,
			Project: "myapp",
			Tests: []Test{
				{
					Name:   "check-size",
					Expect: Expect{FileSize: &FileSizeCheck{Path: "dist/bundle.js", MinBytes: &min, MaxBytes: &max}},
				},
			},
		}
		if err := Validate(cfg); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing path", func(t *testing.T) {
		cfg := &SmokeConfig{
			Version: 1,
			Project: "myapp",
			Tests: []Test{
				{
					Name:   "check-size",
					Expect: Expect{FileSize: &FileSizeCheck{Path: ""}},
				},
			},
		}
		err := Validate(cfg)
		if err == nil {
			t.Fatal("expected error for missing path")
		}
		if !strings.Contains(err.Error(), "file_size.path is required") {
			t.Errorf("wrong error: %v", err)
		}
	})

	t.Run("min_bytes greater than max_bytes", func(t *testing.T) {
		min := int64(5000)
		max := int64(100)
		cfg := &SmokeConfig{
			Version: 1,
			Project: "myapp",
			Tests: []Test{
				{
					Name:   "check-size",
					Expect: Expect{FileSize: &FileSizeCheck{Path: "dist/bundle.js", MinBytes: &min, MaxBytes: &max}},
				},
			},
		}
		err := Validate(cfg)
		if err == nil {
			t.Fatal("expected error for min > max")
		}
		if !strings.Contains(err.Error(), "min_bytes must be <= max_bytes") {
			t.Errorf("wrong error: %v", err)
		}
	})

	t.Run("standalone assertion works without run command", func(t *testing.T) {
		max := int64(5000)
		cfg := &SmokeConfig{
			Version: 1,
			Project: "myapp",
			Tests: []Test{
				{
					Name:   "check-size",
					Expect: Expect{FileSize: &FileSizeCheck{Path: "dist/bundle.js", MaxBytes: &max}},
				},
			},
		}
		if err := Validate(cfg); err != nil {
			t.Errorf("file_size should be a standalone assertion: %v", err)
		}
	})
}

func TestValidate_Auth(t *testing.T) {
	baseConfig := func(profiles []AuthProfile, fallback string, tests []Test) *SmokeConfig {
		if tests == nil {
			tests = []Test{{Name: "test1", Run: "echo ok", Expect: Expect{ExitCode: intPtr(0)}}}
		}
		return &SmokeConfig{
			Version: 1,
			Project: "myapp",
			Auth:    AuthConfig{Profiles: profiles, Fallback: fallback},
			Tests:   tests,
		}
	}

	t.Run("valid aws profile", func(t *testing.T) {
		cfg := baseConfig([]AuthProfile{
			{Provider: "aws", RoleARN: "arn:aws:iam::123456789012:role/smoke-test"},
		}, "", nil)
		if err := Validate(cfg); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("valid gcp profile", func(t *testing.T) {
		cfg := baseConfig([]AuthProfile{
			{
				Provider:                 "gcp",
				WorkloadIdentityProvider: "projects/123/locations/global/workloadIdentityPools/pool/providers/prov",
				ServiceAccountEmail:      "sa@project.iam.gserviceaccount.com",
			},
		}, "", nil)
		if err := Validate(cfg); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("unsupported provider", func(t *testing.T) {
		cfg := baseConfig([]AuthProfile{
			{Provider: "azure"},
		}, "", nil)
		err := Validate(cfg)
		if err == nil {
			t.Fatal("expected error for unsupported provider")
		}
		if !strings.Contains(err.Error(), "unsupported provider") {
			t.Errorf("wrong error: %v", err)
		}
	})

	t.Run("aws missing role_arn", func(t *testing.T) {
		cfg := baseConfig([]AuthProfile{
			{Provider: "aws"},
		}, "", nil)
		err := Validate(cfg)
		if err == nil {
			t.Fatal("expected error for missing role_arn")
		}
		if !strings.Contains(err.Error(), "aws provider requires role_arn") {
			t.Errorf("wrong error: %v", err)
		}
	})

	t.Run("aws invalid role_arn format", func(t *testing.T) {
		cfg := baseConfig([]AuthProfile{
			{Provider: "aws", RoleARN: "not-an-arn"},
		}, "", nil)
		err := Validate(cfg)
		if err == nil {
			t.Fatal("expected error for invalid role_arn")
		}
		if !strings.Contains(err.Error(), "invalid role_arn format") {
			t.Errorf("wrong error: %v", err)
		}
	})

	t.Run("gcp missing workload_identity_provider", func(t *testing.T) {
		cfg := baseConfig([]AuthProfile{
			{Provider: "gcp", ServiceAccountEmail: "sa@project.iam.gserviceaccount.com"},
		}, "", nil)
		err := Validate(cfg)
		if err == nil {
			t.Fatal("expected error for missing workload_identity_provider")
		}
		if !strings.Contains(err.Error(), "gcp provider requires workload_identity_provider") {
			t.Errorf("wrong error: %v", err)
		}
	})

	t.Run("gcp missing service_account_email", func(t *testing.T) {
		cfg := baseConfig([]AuthProfile{
			{Provider: "gcp", WorkloadIdentityProvider: "projects/123/locations/global/workloadIdentityPools/pool/providers/prov"},
		}, "", nil)
		err := Validate(cfg)
		if err == nil {
			t.Fatal("expected error for missing service_account_email")
		}
		if !strings.Contains(err.Error(), "gcp provider requires service_account_email") {
			t.Errorf("wrong error: %v", err)
		}
	})

	t.Run("invalid gcp_credential_format", func(t *testing.T) {
		cfg := baseConfig([]AuthProfile{
			{
				Provider:                 "gcp",
				WorkloadIdentityProvider: "projects/123/locations/global/workloadIdentityPools/pool/providers/prov",
				ServiceAccountEmail:      "sa@project.iam.gserviceaccount.com",
				GCPCredentialFormat:      "invalid",
			},
		}, "", nil)
		err := Validate(cfg)
		if err == nil {
			t.Fatal("expected error for invalid gcp_credential_format")
		}
		if !strings.Contains(err.Error(), "gcp_credential_format must be env or keyfile") {
			t.Errorf("wrong error: %v", err)
		}
	})

	t.Run("duplicate profile name", func(t *testing.T) {
		cfg := baseConfig([]AuthProfile{
			{Name: "prod", Provider: "aws", RoleARN: "arn:aws:iam::111:role/a"},
			{Name: "prod", Provider: "aws", RoleARN: "arn:aws:iam::222:role/b"},
		}, "", nil)
		err := Validate(cfg)
		if err == nil {
			t.Fatal("expected error for duplicate name")
		}
		if !strings.Contains(err.Error(), "duplicate name") {
			t.Errorf("wrong error: %v", err)
		}
	})

	t.Run("duplicate implicit default name", func(t *testing.T) {
		cfg := baseConfig([]AuthProfile{
			{Provider: "aws", RoleARN: "arn:aws:iam::111:role/a"},
			{Provider: "aws", RoleARN: "arn:aws:iam::222:role/b"},
		}, "", nil)
		err := Validate(cfg)
		if err == nil {
			t.Fatal("expected error for duplicate implicit default names")
		}
		if !strings.Contains(err.Error(), "duplicate name") {
			t.Errorf("wrong error: %v", err)
		}
	})

	t.Run("invalid fallback", func(t *testing.T) {
		cfg := baseConfig([]AuthProfile{
			{Provider: "aws", RoleARN: "arn:aws:iam::123456789012:role/test"},
		}, "ignore", nil)
		err := Validate(cfg)
		if err == nil {
			t.Fatal("expected error for invalid fallback")
		}
		if !strings.Contains(err.Error(), "invalid value") {
			t.Errorf("wrong error: %v", err)
		}
	})

	t.Run("valid fallback env", func(t *testing.T) {
		cfg := baseConfig([]AuthProfile{
			{Provider: "aws", RoleARN: "arn:aws:iam::123456789012:role/test"},
		}, "env", nil)
		if err := Validate(cfg); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid session_duration", func(t *testing.T) {
		cfg := baseConfig([]AuthProfile{
			{Provider: "aws", RoleARN: "arn:aws:iam::123456789012:role/test", SessionDuration: "notaduration"},
		}, "", nil)
		err := Validate(cfg)
		if err == nil {
			t.Fatal("expected error for invalid session_duration")
		}
		if !strings.Contains(err.Error(), "invalid session_duration") {
			t.Errorf("wrong error: %v", err)
		}
	})

	t.Run("session_duration out of range for AWS", func(t *testing.T) {
		cfg := baseConfig([]AuthProfile{
			{Provider: "aws", RoleARN: "arn:aws:iam::123456789012:role/test", SessionDuration: "5m"},
		}, "", nil)
		err := Validate(cfg)
		if err == nil {
			t.Fatal("expected error for duration < 15m")
		}
		if !strings.Contains(err.Error(), "between 15m and 12h") {
			t.Errorf("wrong error: %v", err)
		}
	})

	t.Run("session_duration over 12h for AWS", func(t *testing.T) {
		cfg := baseConfig([]AuthProfile{
			{Provider: "aws", RoleARN: "arn:aws:iam::123456789012:role/test", SessionDuration: "13h"},
		}, "", nil)
		err := Validate(cfg)
		if err == nil {
			t.Fatal("expected error for duration > 12h")
		}
		if !strings.Contains(err.Error(), "between 15m and 12h") {
			t.Errorf("wrong error: %v", err)
		}
	})

	t.Run("test references missing profile", func(t *testing.T) {
		cfg := baseConfig(
			[]AuthProfile{
				{Name: "staging", Provider: "aws", RoleARN: "arn:aws:iam::123456789012:role/test"},
			},
			"",
			[]Test{
				{Name: "test1", Run: "echo ok", Auth: "production", Expect: Expect{ExitCode: intPtr(0)}},
			},
		)
		err := Validate(cfg)
		if err == nil {
			t.Fatal("expected error for missing profile reference")
		}
		if !strings.Contains(err.Error(), "auth profile") {
			t.Errorf("wrong error: %v", err)
		}
	})

	t.Run("test references valid profile", func(t *testing.T) {
		cfg := baseConfig(
			[]AuthProfile{
				{Name: "staging", Provider: "aws", RoleARN: "arn:aws:iam::123456789012:role/test"},
			},
			"",
			[]Test{
				{Name: "test1", Run: "echo ok", Auth: "staging", Expect: Expect{ExitCode: intPtr(0)}},
			},
		)
		if err := Validate(cfg); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("no auth section is valid", func(t *testing.T) {
		cfg := &SmokeConfig{
			Version: 1,
			Project: "myapp",
			Tests:   []Test{{Name: "test1", Run: "echo ok", Expect: Expect{ExitCode: intPtr(0)}}},
		}
		if err := Validate(cfg); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func intPtr(i int) *int { return &i }
