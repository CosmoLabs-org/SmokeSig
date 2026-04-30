package schema

import (
	"strings"
	"testing"
	"time"
)

func TestValidate_LifecycleHooks(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *SmokeConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid lifecycle hooks",
			cfg: &SmokeConfig{
				Version: 1,
				Project: "test",
				Lifecycle: LifecycleConfig{
					BeforeAll: []LifecycleHook{
						{Command: "echo before"},
					},
					AfterAll: []LifecycleHook{
						{Command: "echo after", AlwaysRun: true},
					},
					BeforeEach: []LifecycleHook{
						{Command: "echo before-each", Timeout: Duration{Duration: 30 * time.Second}},
					},
					AfterEach: []LifecycleHook{
						{Command: "echo after-each", EnvPass: true},
					},
				},
				Tests: []Test{{Name: "test1", Run: "true"}},
			},
			wantErr: false,
		},
		{
			name: "before_all missing command",
			cfg: &SmokeConfig{
				Version: 1,
				Project: "test",
				Lifecycle: LifecycleConfig{
					BeforeAll: []LifecycleHook{
						{Timeout: Duration{Duration: 30 * time.Second}},
					},
				},
				Tests: []Test{{Name: "test1", Run: "true"}},
			},
			wantErr: true,
			errMsg:  "lifecycle.before_all[0]: command is required",
		},
		{
			name: "after_all missing command",
			cfg: &SmokeConfig{
				Version: 1,
				Project: "test",
				Lifecycle: LifecycleConfig{
					AfterAll: []LifecycleHook{
						{AlwaysRun: true},
					},
				},
				Tests: []Test{{Name: "test1", Run: "true"}},
			},
			wantErr: true,
			errMsg:  "lifecycle.after_all[0]: command is required",
		},
		{
			name: "before_each missing command",
			cfg: &SmokeConfig{
				Version: 1,
				Project: "test",
				Lifecycle: LifecycleConfig{
					BeforeEach: []LifecycleHook{
						{EnvPass: true},
					},
				},
				Tests: []Test{{Name: "test1", Run: "true"}},
			},
			wantErr: true,
			errMsg:  "lifecycle.before_each[0]: command is required",
		},
		{
			name: "after_each missing command",
			cfg: &SmokeConfig{
				Version: 1,
				Project: "test",
				Lifecycle: LifecycleConfig{
					AfterEach: []LifecycleHook{
						{Timeout: Duration{Duration: 30 * time.Second}},
					},
				},
				Tests: []Test{{Name: "test1", Run: "true"}},
			},
			wantErr: true,
			errMsg:  "lifecycle.after_each[0]: command is required",
		},
		{
			name: "empty lifecycle is valid",
			cfg: &SmokeConfig{
				Version:   1,
				Project:   "test",
				Lifecycle: LifecycleConfig{},
				Tests:     []Test{{Name: "test1", Run: "true"}},
			},
			wantErr: false,
		},
		{
			name: "multiple hooks with one missing command",
			cfg: &SmokeConfig{
				Version: 1,
				Project: "test",
				Lifecycle: LifecycleConfig{
					BeforeAll: []LifecycleHook{
						{Command: "echo first"},
						{Command: ""},
					},
				},
				Tests: []Test{{Name: "test1", Run: "true"}},
			},
			wantErr: true,
			errMsg:  "lifecycle.before_all[1]: command is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if errStr := err.Error(); !strings.Contains(errStr, tt.errMsg) {
					t.Errorf("Validate() error = %v, want contain %q", err, tt.errMsg)
				}
			}
		})
	}
}
