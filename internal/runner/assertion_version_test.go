package runner

import (
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

func TestCheckVersion_Match(t *testing.T) {
	result := CheckVersion(&schema.VersionCheck{
		Command: "echo 'go version go1.22.0 linux/amd64'",
		Pattern: `go1\.2[0-9]`,
	})
	if !result.Passed {
		t.Errorf("expected match, got: %s", result.Actual)
	}
}

func TestCheckVersion_NoMatch(t *testing.T) {
	result := CheckVersion(&schema.VersionCheck{
		Command: "echo 'node v18.0.0'",
		Pattern: `v20\.[0-9]+`,
	})
	if result.Passed {
		t.Error("expected no match")
	}
}

func TestCheckVersion_CommandFailure(t *testing.T) {
	result := CheckVersion(&schema.VersionCheck{
		Command: "false",
		Pattern: `.*`,
	})
	if result.Passed {
		t.Error("expected fail for non-zero exit")
	}
}
