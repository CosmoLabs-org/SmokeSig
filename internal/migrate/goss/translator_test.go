package goss

import (
	"os"
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

func TestTranslateProcess(t *testing.T) {
	gf := mustParse(t, "testdata/goss/basic.yaml")
	tests, warnings := Translate(gf, TranslateOptions{})

	// Should have process tests
	processTests := filterTests(tests, "process:")
	if len(processTests) != 2 {
		t.Fatalf("process tests = %d, want 2", len(processTests))
	}

	// Should use native process_running assertion
	found := false
	for _, tt := range processTests {
		if tt.Expect.ProcessRunning == "nginx" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected process_running assertion for nginx")
	}

	// No warnings for direct mapping
	for _, w := range warnings {
		if w.GossKey == "process" {
			t.Errorf("process should not produce warnings, got: %s", w.Message)
		}
	}
}

func TestTranslatePort(t *testing.T) {
	gf := mustParse(t, "testdata/goss/basic.yaml")
	tests, _ := Translate(gf, TranslateOptions{})

	portTests := filterTests(tests, "port:")
	if len(portTests) != 3 {
		t.Fatalf("port tests = %d, want 3", len(portTests))
	}

	// Verify tcp:80 has port_listening
	for _, tt := range portTests {
		if tt.Expect.PortListening != nil && tt.Expect.PortListening.Port == 80 {
			if tt.Expect.PortListening.Protocol != "tcp" {
				t.Error("tcp:80 should have protocol tcp")
			}
			break
		}
	}
}

func TestTranslateCommand(t *testing.T) {
	gf := mustParse(t, "testdata/goss/basic.yaml")
	tests, _ := Translate(gf, TranslateOptions{})

	cmdTests := filterTests(tests, "command:")
	if len(cmdTests) != 1 {
		t.Fatalf("command tests = %d, want 1", len(cmdTests))
	}

	// Command should be the run target
	if cmdTests[0].Run != "echo hello" {
		t.Errorf("command run = %q, want %q", cmdTests[0].Run, "echo hello")
	}

	// Should map exit-status and stdout
	if cmdTests[0].Expect.ExitCode == nil || *cmdTests[0].Expect.ExitCode != 0 {
		t.Error("expected exit_code 0")
	}
	if cmdTests[0].Expect.StdoutContains != "hello" {
		t.Errorf("stdout_contains = %q, want %q", cmdTests[0].Expect.StdoutContains, "hello")
	}
}

func TestTranslateFile(t *testing.T) {
	gf := mustParse(t, "testdata/goss/basic.yaml")
	tests, warnings := Translate(gf, TranslateOptions{})

	fileTests := filterTests(tests, "file:")
	if len(fileTests) != 2 {
		t.Fatalf("file tests = %d, want 2", len(fileTests))
	}

	// /etc/hosts should have file_exists
	found := false
	for _, tt := range fileTests {
		if tt.Expect.FileExists == "/etc/hosts" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected file_exists assertion for /etc/hosts")
	}

	// Should warn about unmapped attrs (mode, owner)
	partialWarns := filterWarnings(warnings, "file")
	if len(partialWarns) == 0 {
		t.Error("expected warnings for unmapped file attributes (mode, owner)")
	}
}

func TestTranslateHTTP(t *testing.T) {
	gf := mustParse(t, "testdata/goss/basic.yaml")
	tests, _ := Translate(gf, TranslateOptions{})

	httpTests := filterTests(tests, "http:")
	if len(httpTests) != 1 {
		t.Fatalf("http tests = %d, want 1", len(httpTests))
	}

	if httpTests[0].Expect.HTTP == nil {
		t.Fatal("expected HTTP check")
	}
	if httpTests[0].Expect.HTTP.URL != "http://localhost:80/health" {
		t.Errorf("http url = %q", httpTests[0].Expect.HTTP.URL)
	}
	if httpTests[0].Expect.HTTP.StatusCode == nil || *httpTests[0].Expect.HTTP.StatusCode != 200 {
		t.Error("expected status_code 200")
	}
	if httpTests[0].Expect.HTTP.BodyContains != "ok" {
		t.Errorf("body_contains = %q, want %q", httpTests[0].Expect.HTTP.BodyContains, "ok")
	}
}

func TestTranslatePackage(t *testing.T) {
	gf := mustParse(t, "testdata/goss/basic.yaml")

	// Default distro (deb)
	tests, _ := Translate(gf, TranslateOptions{Distro: "deb"})
	pkgTests := filterTests(tests, "package:")
	if len(pkgTests) != 2 {
		t.Fatalf("package tests = %d, want 2", len(pkgTests))
	}

	// Verify dpkg command
	found := false
	for _, tt := range pkgTests {
		if tt.Run == "dpkg -l nginx | grep ^ii" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected dpkg command for nginx")
	}

	// RPM distro
	tests, _ = Translate(gf, TranslateOptions{Distro: "rpm"})
	for _, tt := range tests {
		if tt.Name == "package:nginx installed" {
			if tt.Run != "rpm -q nginx" {
				t.Errorf("rpm run = %q, want %q", tt.Run, "rpm -q nginx")
			}
			break
		}
	}

	// APK distro
	tests, _ = Translate(gf, TranslateOptions{Distro: "apk"})
	for _, tt := range tests {
		if tt.Name == "package:nginx installed" {
			if tt.Run != "apk info -e nginx" {
				t.Errorf("apk run = %q, want %q", tt.Run, "apk info -e nginx")
			}
			break
		}
	}
}

func TestTranslateService(t *testing.T) {
	gf := mustParse(t, "testdata/goss/basic.yaml")
	tests, _ := Translate(gf, TranslateOptions{})

	svcTests := filterTests(tests, "service:")
	if len(svcTests) != 2 {
		t.Fatalf("service tests = %d, want 2 (running + enabled)", len(svcTests))
	}

	// Should have both active and enabled checks
	foundActive, foundEnabled := false, false
	for _, tt := range svcTests {
		if tt.Name == "service:nginx active" {
			foundActive = true
		}
		if tt.Name == "service:nginx enabled" {
			foundEnabled = true
		}
	}
	if !foundActive {
		t.Error("missing service:nginx active test")
	}
	if !foundEnabled {
		t.Error("missing service:nginx enabled test")
	}
}

func TestTranslateLongtail(t *testing.T) {
	gf := mustParse(t, "testdata/goss/longtail.yaml")
	tests, warnings := Translate(gf, TranslateOptions{})

	// All long-tail keys should produce tests (not panic)
	if len(tests) == 0 {
		t.Fatal("longtail translation produced no tests")
	}

	// Should have warnings for command fallback
	if len(warnings) == 0 {
		t.Error("longtail keys should produce warnings")
	}

	// Verify specific mappings
	userTests := filterTests(tests, "user:")
	if len(userTests) != 1 {
		t.Errorf("user tests = %d, want 1", len(userTests))
	}

	kernelTests := filterTests(tests, "kernel-param:")
	if len(kernelTests) != 1 {
		t.Errorf("kernel-param tests = %d, want 1", len(kernelTests))
	}
	if kernelTests[0].Expect.StdoutContains != "1" {
		t.Errorf("kernel-param stdout_contains = %q, want %q", kernelTests[0].Expect.StdoutContains, "1")
	}
}

func TestTranslateEmpty(t *testing.T) {
	gf := &GossFile{}
	tests, warnings := Translate(gf, TranslateOptions{})

	if len(tests) != 0 {
		t.Errorf("empty gossfile should produce 0 tests, got %d", len(tests))
	}
	if len(warnings) != 0 {
		t.Errorf("empty gossfile should produce 0 warnings, got %d", len(warnings))
	}
}

func TestEmittedOutputParsesBack(t *testing.T) {
	gf := mustParse(t, "testdata/goss/basic.yaml")
	tests, warnings := Translate(gf, TranslateOptions{})

	output, err := Emit(tests, warnings, EmitMeta{Source: "testdata/goss/basic.yaml"})
	if err != nil {
		t.Fatalf("Emit() error = %v", err)
	}

	cfg, err := schema.Parse([]byte(output))
	if err != nil {
		t.Fatalf("generated output should parse as valid .smokesig.yaml: %v\nOutput:\n%s", err, output)
	}
	if len(cfg.Tests) != len(tests) {
		t.Errorf("parsed tests = %d, want %d", len(cfg.Tests), len(tests))
	}
}

func TestEmittedOutputParsesBackLongtail(t *testing.T) {
	gf := mustParse(t, "testdata/goss/longtail.yaml")
	tests, warnings := Translate(gf, TranslateOptions{})

	output, err := Emit(tests, warnings, EmitMeta{Source: "testdata/goss/longtail.yaml"})
	if err != nil {
		t.Fatalf("Emit() error = %v", err)
	}

	cfg, err := schema.Parse([]byte(output))
	if err != nil {
		t.Fatalf("generated output should parse as valid .smokesig.yaml: %v\nOutput:\n%s", err, output)
	}
	if len(cfg.Tests) == 0 {
		t.Error("expected at least one test from longtail translation")
	}
}

func TestEmitStats(t *testing.T) {
	warnings := []TranslationWarning{
		{GossKey: "service", Resource: "nginx", Category: WarnCommandFallback, Message: "fallback"},
		{GossKey: "dns", Resource: "example.com", Category: WarnCommandFallback, Message: "fallback"},
		{GossKey: "file", Resource: "/etc/hosts", Category: WarnPartial, Message: "partial"},
	}
	stats := EmitStats(warnings)
	if stats == "" {
		t.Error("EmitStats should return non-empty string")
	}
}

func TestPortParsingEdgeCases(t *testing.T) {
	tests := []struct {
		input    string
		wantPort int
		wantProto string
		wantHost string
	}{
		{"tcp:80", 80, "tcp", "0.0.0.0"},
		{"udp:53", 53, "udp", "0.0.0.0"},
		{"tcp:8080:127.0.0.1", 8080, "tcp", "127.0.0.1"},
	}

	for _, tc := range tests {
		gf := &GossFile{
			Port: map[string]GossAttrs{
				tc.input: {"listening": true},
			},
		}
		tests, warnings := Translate(gf, TranslateOptions{})
		if len(warnings) > 0 {
			t.Errorf("port %q: unexpected warnings: %v", tc.input, warnings)
			continue
		}
		if len(tests) != 1 {
			t.Errorf("port %q: got %d tests, want 1", tc.input, len(tests))
			continue
		}
		pl := tests[0].Expect.PortListening
		if pl == nil {
			t.Errorf("port %q: nil PortListening", tc.input)
			continue
		}
		if pl.Port != tc.wantPort {
			t.Errorf("port %q: port = %d, want %d", tc.input, pl.Port, tc.wantPort)
		}
		if pl.Protocol != tc.wantProto {
			t.Errorf("port %q: protocol = %q, want %q", tc.input, pl.Protocol, tc.wantProto)
		}
		if pl.Host != tc.wantHost {
			t.Errorf("port %q: host = %q, want %q", tc.input, pl.Host, tc.wantHost)
		}
	}
}

func TestServiceOnlyRunning(t *testing.T) {
	gf := &GossFile{
		Service: map[string]GossAttrs{
			"sshd": {"running": true},
		},
	}
	tests, _ := Translate(gf, TranslateOptions{})
	svcTests := filterTests(tests, "service:")
	if len(svcTests) != 1 {
		t.Fatalf("service tests = %d, want 1 (running only)", len(svcTests))
	}
	if svcTests[0].Name != "service:sshd active" {
		t.Errorf("service name = %q", svcTests[0].Name)
	}
}

func TestPackageNotInstalled(t *testing.T) {
	gf := &GossFile{
		Package: map[string]GossAttrs{
			"vim": {"installed": false},
		},
	}
	tests, _ := Translate(gf, TranslateOptions{})
	pkgTests := filterTests(tests, "package:")
	if len(pkgTests) != 0 {
		t.Errorf("installed:false should produce 0 tests, got %d", len(pkgTests))
	}
}

// Helpers

func mustParse(t *testing.T, path string) *GossFile {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	gf, err := Parse(data)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	return gf
}

func filterTests(tests []schema.Test, prefix string) []schema.Test {
	var result []schema.Test
	for _, tt := range tests {
		if len(tt.Name) >= len(prefix) && tt.Name[:len(prefix)] == prefix {
			result = append(result, tt)
		}
	}
	return result
}

func filterWarnings(warnings []TranslationWarning, gossKey string) []TranslationWarning {
	var result []TranslationWarning
	for _, w := range warnings {
		if w.GossKey == gossKey {
			result = append(result, w)
		}
	}
	return result
}
