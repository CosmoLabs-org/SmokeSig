package goss

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// intVal — uncovered branches: nil interface, string-to-int, float-to-int
// ---------------------------------------------------------------------------

func TestIntValNilInterface(t *testing.T) {
	m := map[string]interface{}{"key": nil}
	if got := intVal(m, "key"); got != 0 {
		t.Errorf("intVal(nil interface) = %d, want 0", got)
	}
}

func TestIntValMissingKey(t *testing.T) {
	m := map[string]interface{}{}
	if got := intVal(m, "missing"); got != 0 {
		t.Errorf("intVal(missing key) = %d, want 0", got)
	}
}

func TestIntValStringToInt(t *testing.T) {
	m := map[string]interface{}{"port": "8080"}
	if got := intVal(m, "port"); got != 8080 {
		t.Errorf("intVal(string) = %d, want 8080", got)
	}
}

func TestIntValStringInvalid(t *testing.T) {
	m := map[string]interface{}{"port": "notanint"}
	if got := intVal(m, "port"); got != 0 {
		t.Errorf("intVal(invalid string) = %d, want 0", got)
	}
}

func TestIntValFloat64(t *testing.T) {
	m := map[string]interface{}{"status": float64(200)}
	if got := intVal(m, "status"); got != 200 {
		t.Errorf("intVal(float64) = %d, want 200", got)
	}
}

func TestIntValInt(t *testing.T) {
	m := map[string]interface{}{"exit": int(42)}
	if got := intVal(m, "exit"); got != 42 {
		t.Errorf("intVal(int) = %d, want 42", got)
	}
}

// ---------------------------------------------------------------------------
// translatePort — UDP protocol and invalid port number
// ---------------------------------------------------------------------------

func TestTranslatePortUDP(t *testing.T) {
	gf := &GossFile{
		Port: map[string]GossAttrs{
			"udp:53": {"listening": true},
		},
	}
	tests, warnings := Translate(gf, TranslateOptions{})
	if len(warnings) > 0 {
		t.Errorf("udp:53: unexpected warnings: %v", warnings)
	}
	if len(tests) != 1 {
		t.Fatalf("udp:53: got %d tests, want 1", len(tests))
	}
	pl := tests[0].Expect.PortListening
	if pl == nil {
		t.Fatal("udp:53: nil PortListening")
	}
	if pl.Protocol != "udp" {
		t.Errorf("protocol = %q, want udp", pl.Protocol)
	}
	if pl.Port != 53 {
		t.Errorf("port = %d, want 53", pl.Port)
	}
}

func TestTranslatePortInvalidPortNumber(t *testing.T) {
	gf := &GossFile{
		Port: map[string]GossAttrs{
			"tcp:notaport": {"listening": true},
		},
	}
	tests, warnings := Translate(gf, TranslateOptions{})
	if len(tests) != 0 {
		t.Errorf("invalid port: expected 0 tests, got %d", len(tests))
	}
	if len(warnings) == 0 {
		t.Error("invalid port: expected a warning")
	}
}

func TestTranslatePortMissingPortParts(t *testing.T) {
	gf := &GossFile{
		Port: map[string]GossAttrs{
			"nocolon": {"listening": true},
		},
	}
	tests, warnings := Translate(gf, TranslateOptions{})
	if len(tests) != 0 {
		t.Errorf("no-colon port: expected 0 tests, got %d", len(tests))
	}
	if len(warnings) == 0 {
		t.Error("no-colon port: expected a warning")
	}
}

func TestTranslatePortNotListening(t *testing.T) {
	gf := &GossFile{
		Port: map[string]GossAttrs{
			"tcp:80": {"listening": false},
		},
	}
	tests, _ := Translate(gf, TranslateOptions{})
	if len(tests) != 0 {
		t.Errorf("listening:false: expected 0 tests, got %d", len(tests))
	}
}

func TestTranslatePortWithIPField(t *testing.T) {
	gf := &GossFile{
		Port: map[string]GossAttrs{
			"tcp:443": {
				"listening": true,
				"ip":        []interface{}{"127.0.0.1"},
			},
		},
	}
	tests, _ := Translate(gf, TranslateOptions{})
	if len(tests) != 1 {
		t.Fatalf("port with ip: got %d tests, want 1", len(tests))
	}
	if tests[0].Expect.PortListening.Host != "127.0.0.1" {
		t.Errorf("host = %q, want 127.0.0.1", tests[0].Expect.PortListening.Host)
	}
}

// ---------------------------------------------------------------------------
// translateCommand — exit-status explicitly provided
// ---------------------------------------------------------------------------

func TestTranslateCommandWithExitStatus(t *testing.T) {
	gf := &GossFile{
		Command: map[string]GossAttrs{
			"false": {"exit-status": 1},
		},
	}
	tests, _ := Translate(gf, TranslateOptions{})
	if len(tests) != 1 {
		t.Fatalf("command with exit-status: got %d tests, want 1", len(tests))
	}
	if tests[0].Expect.ExitCode == nil {
		t.Fatal("expected non-nil ExitCode")
	}
	if *tests[0].Expect.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", *tests[0].Expect.ExitCode)
	}
}

func TestTranslateCommandWithZeroExitStatusExplicit(t *testing.T) {
	gf := &GossFile{
		Command: map[string]GossAttrs{
			"true": {"exit-status": 0},
		},
	}
	tests, _ := Translate(gf, TranslateOptions{})
	if len(tests) != 1 {
		t.Fatalf("command exit-status 0: got %d tests, want 1", len(tests))
	}
	// hasKey returns true, so ExitCode should be set
	if tests[0].Expect.ExitCode == nil || *tests[0].Expect.ExitCode != 0 {
		t.Error("expected ExitCode=0 when exit-status key present")
	}
}

func TestTranslateCommandWithStderrOnly(t *testing.T) {
	gf := &GossFile{
		Command: map[string]GossAttrs{
			"ls /nonexistent": {"stderr": []interface{}{"No such file"}},
		},
	}
	tests, _ := Translate(gf, TranslateOptions{})
	if len(tests) != 1 {
		t.Fatalf("got %d tests, want 1", len(tests))
	}
	if tests[0].Expect.StderrContains != "No such file" {
		t.Errorf("StderrContains = %q, want %q", tests[0].Expect.StderrContains, "No such file")
	}
}

func TestTranslateCommandMultipleStdoutWarning(t *testing.T) {
	gf := &GossFile{
		Command: map[string]GossAttrs{
			"echo test": {
				"stdout": []interface{}{"line1", "line2"},
			},
		},
	}
	tests, warnings := Translate(gf, TranslateOptions{})
	if len(tests) != 1 {
		t.Fatalf("got %d tests, want 1", len(tests))
	}
	// Should warn about multiple stdout entries
	found := false
	for _, w := range warnings {
		if w.GossKey == "command" && w.Category == WarnPartial {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected WarnPartial warning for multiple stdout entries")
	}
	// Only first stdout is mapped
	if tests[0].Expect.StdoutContains != "line1" {
		t.Errorf("StdoutContains = %q, want %q", tests[0].Expect.StdoutContains, "line1")
	}
}

// ---------------------------------------------------------------------------
// translateFile — owner, mode, sha256/checksum attrs
// ---------------------------------------------------------------------------

func TestTranslateFileWithOwnerAndMode(t *testing.T) {
	gf := &GossFile{
		File: map[string]GossAttrs{
			"/etc/passwd": {
				"exists": true,
				"owner":  "root",
				"mode":   "0644",
			},
		},
	}
	tests, warnings := Translate(gf, TranslateOptions{})
	// Should still produce the file_exists test
	if len(tests) != 1 {
		t.Fatalf("got %d tests, want 1", len(tests))
	}
	if tests[0].Expect.FileExists != "/etc/passwd" {
		t.Errorf("FileExists = %q", tests[0].Expect.FileExists)
	}
	// Should warn about unmapped attrs
	found := false
	for _, w := range warnings {
		if w.GossKey == "file" && w.Category == WarnPartial {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected WarnPartial warning for owner/mode attrs")
	}
}

func TestTranslateFileWithChecksum(t *testing.T) {
	gf := &GossFile{
		File: map[string]GossAttrs{
			"/usr/bin/curl": {
				"exists":   true,
				"checksum": "sha256:abc123",
			},
		},
	}
	tests, warnings := Translate(gf, TranslateOptions{})
	if len(tests) != 1 {
		t.Fatalf("got %d tests, want 1", len(tests))
	}
	partialWarns := filterWarnings(warnings, "file")
	if len(partialWarns) == 0 {
		t.Error("expected warning for unmapped checksum attr")
	}
	// Warning message should mention checksum
	found := false
	for _, w := range partialWarns {
		if strings.Contains(w.Message, "checksum") {
			found = true
			break
		}
	}
	if !found {
		t.Error("warning message should mention 'checksum'")
	}
}

func TestTranslateFileWithAllUnmappedAttrs(t *testing.T) {
	gf := &GossFile{
		File: map[string]GossAttrs{
			"/tmp/test": {
				"exists":   true,
				"owner":    "root",
				"group":    "root",
				"mode":     "0755",
				"contains": []interface{}{"something"},
				"filetype": "file",
				"size":     1024,
			},
		},
	}
	_, warnings := Translate(gf, TranslateOptions{})
	if len(warnings) == 0 {
		t.Error("expected warnings for unmapped file attrs")
	}
	// Warning should mention multiple attrs
	msg := warnings[0].Message
	if !strings.Contains(msg, "owner") || !strings.Contains(msg, "mode") {
		t.Errorf("warning should mention owner and mode, got: %q", msg)
	}
}

func TestTranslateFileNotExists(t *testing.T) {
	gf := &GossFile{
		File: map[string]GossAttrs{
			"/nonexistent": {"exists": false},
		},
	}
	tests, _ := Translate(gf, TranslateOptions{})
	if len(tests) != 0 {
		t.Errorf("exists:false should produce 0 tests, got %d", len(tests))
	}
}

// ---------------------------------------------------------------------------
// translatePackage — with version attr
// ---------------------------------------------------------------------------

func TestTranslatePackageWithVersion(t *testing.T) {
	gf := &GossFile{
		Package: map[string]GossAttrs{
			"curl": {
				"installed": true,
				"version":   "7.68.0",
			},
		},
	}
	tests, warnings := Translate(gf, TranslateOptions{Distro: "deb"})
	if len(tests) != 1 {
		t.Fatalf("got %d tests, want 1", len(tests))
	}
	// Should produce a warning about the version not being mapped
	found := false
	for _, w := range warnings {
		if w.GossKey == "package" && w.Category == WarnPartial {
			found = true
			if !strings.Contains(w.Message, "7.68.0") {
				t.Errorf("warning message should mention version, got: %q", w.Message)
			}
			break
		}
	}
	if !found {
		t.Error("expected WarnPartial warning for package version")
	}
}

// ---------------------------------------------------------------------------
// translateUser — not exists path
// ---------------------------------------------------------------------------

func TestTranslateUserExists(t *testing.T) {
	gf := &GossFile{
		User: map[string]GossAttrs{
			"www-data": {"exists": true},
		},
	}
	tests, warnings := Translate(gf, TranslateOptions{})
	if len(tests) != 1 {
		t.Fatalf("user exists: got %d tests, want 1", len(tests))
	}
	if tests[0].Name != "user:www-data exists" {
		t.Errorf("name = %q", tests[0].Name)
	}
	if tests[0].Run != "id www-data" {
		t.Errorf("run = %q, want %q", tests[0].Run, "id www-data")
	}
	if len(warnings) == 0 {
		t.Error("expected command fallback warning for user")
	}
}

func TestTranslateUserNotExists(t *testing.T) {
	gf := &GossFile{
		User: map[string]GossAttrs{
			"ghost": {"exists": false},
		},
	}
	tests, _ := Translate(gf, TranslateOptions{})
	if len(tests) != 0 {
		t.Errorf("user exists:false: expected 0 tests, got %d", len(tests))
	}
}

func TestTranslateUserWithExtraAttrs(t *testing.T) {
	// Extra attrs like uid, groups, home, shell are present but don't change translation
	gf := &GossFile{
		User: map[string]GossAttrs{
			"deploy": {
				"exists": true,
				"uid":    1001,
				"groups": []interface{}{"sudo", "docker"},
				"home":   "/home/deploy",
				"shell":  "/bin/bash",
			},
		},
	}
	tests, warnings := Translate(gf, TranslateOptions{})
	if len(tests) != 1 {
		t.Fatalf("user with extra attrs: got %d tests, want 1", len(tests))
	}
	// Should still use id command
	if tests[0].Run != "id deploy" {
		t.Errorf("run = %q", tests[0].Run)
	}
	if len(warnings) == 0 {
		t.Error("expected command fallback warning")
	}
}

// ---------------------------------------------------------------------------
// translateGroup — not exists path + with gid
// ---------------------------------------------------------------------------

func TestTranslateGroupExists(t *testing.T) {
	gf := &GossFile{
		Group: map[string]GossAttrs{
			"docker": {"exists": true},
		},
	}
	tests, warnings := Translate(gf, TranslateOptions{})
	if len(tests) != 1 {
		t.Fatalf("group exists: got %d tests, want 1", len(tests))
	}
	if tests[0].Run != "getent group docker" {
		t.Errorf("run = %q", tests[0].Run)
	}
	if len(warnings) == 0 {
		t.Error("expected command fallback warning")
	}
}

func TestTranslateGroupNotExists(t *testing.T) {
	gf := &GossFile{
		Group: map[string]GossAttrs{
			"nobody": {"exists": false},
		},
	}
	tests, _ := Translate(gf, TranslateOptions{})
	if len(tests) != 0 {
		t.Errorf("group exists:false: expected 0 tests, got %d", len(tests))
	}
}

func TestTranslateGroupWithGID(t *testing.T) {
	gf := &GossFile{
		Group: map[string]GossAttrs{
			"wheel": {
				"exists": true,
				"gid":    10,
			},
		},
	}
	tests, _ := Translate(gf, TranslateOptions{})
	if len(tests) != 1 {
		t.Fatalf("group with gid: got %d tests, want 1", len(tests))
	}
	// gid is not mapped — just a passthrough, check test is still generated
	if tests[0].Name != "group:wheel exists" {
		t.Errorf("name = %q", tests[0].Name)
	}
}

// ---------------------------------------------------------------------------
// translateDNS — not resolvable, with addrs/timeout
// ---------------------------------------------------------------------------

func TestTranslateDNSResolvable(t *testing.T) {
	gf := &GossFile{
		DNS: map[string]GossAttrs{
			"example.com": {"resolvable": true},
		},
	}
	tests, warnings := Translate(gf, TranslateOptions{})
	if len(tests) != 1 {
		t.Fatalf("dns resolvable: got %d tests, want 1", len(tests))
	}
	if tests[0].Name != "dns:example.com resolvable" {
		t.Errorf("name = %q", tests[0].Name)
	}
	if len(warnings) == 0 {
		t.Error("expected command fallback warning")
	}
}

func TestTranslateDNSNotResolvable(t *testing.T) {
	gf := &GossFile{
		DNS: map[string]GossAttrs{
			"example.com": {"resolvable": false},
		},
	}
	tests, _ := Translate(gf, TranslateOptions{})
	if len(tests) != 0 {
		t.Errorf("dns resolvable:false: expected 0 tests, got %d", len(tests))
	}
}

func TestTranslateDNSWithAddrsAndTimeout(t *testing.T) {
	// addrs and timeout are extra attrs — not crash, still generates test
	gf := &GossFile{
		DNS: map[string]GossAttrs{
			"api.internal": {
				"resolvable": true,
				"addrs":      []interface{}{"10.0.0.1"},
				"timeout":    "2s",
			},
		},
	}
	tests, warnings := Translate(gf, TranslateOptions{})
	if len(tests) != 1 {
		t.Fatalf("dns with addrs+timeout: got %d tests, want 1", len(tests))
	}
	if len(warnings) == 0 {
		t.Error("expected warning for dns translation")
	}
}

// ---------------------------------------------------------------------------
// translateAddr — with/without port, timeout, local-address
// ---------------------------------------------------------------------------

func TestTranslateAddrWithPort(t *testing.T) {
	gf := &GossFile{
		Addr: map[string]GossAttrs{
			"tcp://db.internal:5432": {"reachable": true},
		},
	}
	tests, warnings := Translate(gf, TranslateOptions{})
	if len(tests) != 1 {
		t.Fatalf("addr with port: got %d tests, want 1", len(tests))
	}
	if len(warnings) != 0 {
		t.Errorf("addr with port: unexpected warnings: %v", warnings)
	}
	if tests[0].Expect.PortListening == nil {
		t.Fatal("expected PortListening for addr with port")
	}
	if tests[0].Expect.PortListening.Port != 5432 {
		t.Errorf("port = %d, want 5432", tests[0].Expect.PortListening.Port)
	}
}

func TestTranslateAddrWithoutPort(t *testing.T) {
	gf := &GossFile{
		Addr: map[string]GossAttrs{
			"somehost": {"reachable": true},
		},
	}
	tests, warnings := Translate(gf, TranslateOptions{})
	if len(tests) != 1 {
		t.Fatalf("addr without port: got %d tests, want 1", len(tests))
	}
	if len(warnings) == 0 {
		t.Error("expected command fallback warning for addr without port")
	}
}

func TestTranslateAddrNotReachable(t *testing.T) {
	gf := &GossFile{
		Addr: map[string]GossAttrs{
			"tcp://example.com:80": {"reachable": false},
		},
	}
	tests, _ := Translate(gf, TranslateOptions{})
	if len(tests) != 0 {
		t.Errorf("reachable:false: expected 0 tests, got %d", len(tests))
	}
}

func TestTranslateAddrWithTimeout(t *testing.T) {
	gf := &GossFile{
		Addr: map[string]GossAttrs{
			"tcp://redis.internal:6379": {
				"reachable": true,
				"timeout":   "500ms",
			},
		},
	}
	tests, _ := Translate(gf, TranslateOptions{})
	if len(tests) != 1 {
		t.Fatalf("addr with timeout: got %d tests, want 1", len(tests))
	}
	if tests[0].Expect.PortListening == nil {
		t.Fatal("expected PortListening")
	}
	if tests[0].Expect.PortListening.Port != 6379 {
		t.Errorf("port = %d, want 6379", tests[0].Expect.PortListening.Port)
	}
}

func TestTranslateAddrUDP(t *testing.T) {
	gf := &GossFile{
		Addr: map[string]GossAttrs{
			"udp://dns.server:53": {"reachable": true},
		},
	}
	tests, _ := Translate(gf, TranslateOptions{})
	if len(tests) != 1 {
		t.Fatalf("udp addr: got %d tests, want 1", len(tests))
	}
}

// ---------------------------------------------------------------------------
// translateInterface — not exists, with addrs
// ---------------------------------------------------------------------------

func TestTranslateInterfaceExists(t *testing.T) {
	gf := &GossFile{
		Interface: map[string]GossAttrs{
			"lo": {"exists": true},
		},
	}
	tests, warnings := Translate(gf, TranslateOptions{})
	if len(tests) != 1 {
		t.Fatalf("interface exists: got %d tests, want 1", len(tests))
	}
	if tests[0].Run != "ip link show lo" {
		t.Errorf("run = %q", tests[0].Run)
	}
	if len(warnings) == 0 {
		t.Error("expected command fallback warning")
	}
}

func TestTranslateInterfaceNotExists(t *testing.T) {
	gf := &GossFile{
		Interface: map[string]GossAttrs{
			"eth0": {"exists": false},
		},
	}
	tests, _ := Translate(gf, TranslateOptions{})
	if len(tests) != 0 {
		t.Errorf("interface exists:false: expected 0 tests, got %d", len(tests))
	}
}

func TestTranslateInterfaceWithAddrs(t *testing.T) {
	gf := &GossFile{
		Interface: map[string]GossAttrs{
			"eth0": {
				"exists": true,
				"addrs":  []interface{}{"192.168.1.1/24"},
			},
		},
	}
	tests, warnings := Translate(gf, TranslateOptions{})
	if len(tests) != 1 {
		t.Fatalf("interface with addrs: got %d tests, want 1", len(tests))
	}
	if len(warnings) == 0 {
		t.Error("expected command fallback warning")
	}
}

// ---------------------------------------------------------------------------
// translateMount — not exists, with opts/vfs_opts
// ---------------------------------------------------------------------------

func TestTranslateMountExists(t *testing.T) {
	gf := &GossFile{
		Mount: map[string]GossAttrs{
			"/mnt/data": {"exists": true},
		},
	}
	tests, warnings := Translate(gf, TranslateOptions{})
	if len(tests) != 1 {
		t.Fatalf("mount exists: got %d tests, want 1", len(tests))
	}
	if tests[0].Run != "mountpoint -q /mnt/data" {
		t.Errorf("run = %q", tests[0].Run)
	}
	if len(warnings) == 0 {
		t.Error("expected command fallback warning")
	}
}

func TestTranslateMountNotExists(t *testing.T) {
	gf := &GossFile{
		Mount: map[string]GossAttrs{
			"/mnt/data": {"exists": false},
		},
	}
	tests, _ := Translate(gf, TranslateOptions{})
	if len(tests) != 0 {
		t.Errorf("mount exists:false: expected 0 tests, got %d", len(tests))
	}
}

func TestTranslateMountWithOpts(t *testing.T) {
	gf := &GossFile{
		Mount: map[string]GossAttrs{
			"/boot": {
				"exists":   true,
				"opts":     []interface{}{"rw", "relatime"},
				"vfs-opts": []interface{}{"rw"},
			},
		},
	}
	tests, _ := Translate(gf, TranslateOptions{})
	if len(tests) != 1 {
		t.Fatalf("mount with opts: got %d tests, want 1", len(tests))
	}
	if tests[0].Name != "mount:/boot exists" {
		t.Errorf("name = %q", tests[0].Name)
	}
}

// ---------------------------------------------------------------------------
// translateKernelParam — missing value branch (returns nil)
// ---------------------------------------------------------------------------

func TestTranslateKernelParamWithValue(t *testing.T) {
	gf := &GossFile{
		KernelParam: map[string]GossAttrs{
			"vm.swappiness": {"value": "10"},
		},
	}
	tests, warnings := Translate(gf, TranslateOptions{})
	if len(tests) != 1 {
		t.Fatalf("kernel-param with value: got %d tests, want 1", len(tests))
	}
	if tests[0].Expect.StdoutContains != "10" {
		t.Errorf("stdout_contains = %q, want 10", tests[0].Expect.StdoutContains)
	}
	if len(warnings) == 0 {
		t.Error("expected command fallback warning")
	}
}

func TestTranslateKernelParamNoValue(t *testing.T) {
	gf := &GossFile{
		KernelParam: map[string]GossAttrs{
			"net.ipv4.ip_forward": {},
		},
	}
	tests, _ := Translate(gf, TranslateOptions{})
	if len(tests) != 0 {
		t.Errorf("kernel-param no value: expected 0 tests, got %d", len(tests))
	}
}

// ---------------------------------------------------------------------------
// Stats (emitter.go) — empty warnings
// ---------------------------------------------------------------------------

func TestStatsEmptyWarnings(t *testing.T) {
	direct, cmdFallback, skipped := Stats(nil)
	if direct != 0 || cmdFallback != 0 || skipped != 0 {
		t.Errorf("Stats(nil) = (%d,%d,%d), want (0,0,0)", direct, cmdFallback, skipped)
	}
}

func TestStatsAllCategories(t *testing.T) {
	warnings := []TranslationWarning{
		{Category: WarnCommandFallback},
		{Category: WarnSkipped},
		{Category: WarnPartial},
	}
	direct, cmdFallback, skipped := Stats(warnings)
	if direct != 0 {
		t.Errorf("direct = %d, want 0", direct)
	}
	// WarnCommandFallback + WarnPartial both count as commandFallback
	if cmdFallback != 2 {
		t.Errorf("cmdFallback = %d, want 2", cmdFallback)
	}
	if skipped != 1 {
		t.Errorf("skipped = %d, want 1", skipped)
	}
}

// ---------------------------------------------------------------------------
// Emit (emitter.go) — with warnings, with timestamp
// ---------------------------------------------------------------------------

func TestEmitWithWarnings(t *testing.T) {
	gf := &GossFile{
		Service: map[string]GossAttrs{
			"nginx": {"running": true, "enabled": true},
		},
	}
	tests, warnings := Translate(gf, TranslateOptions{})
	if len(warnings) == 0 {
		t.Fatal("expected warnings from service translation")
	}

	output, err := Emit(tests, warnings, EmitMeta{Source: "test.yaml"})
	if err != nil {
		t.Fatalf("Emit() error = %v", err)
	}
	if output == "" {
		t.Error("Emit() returned empty string")
	}
	// Stats line should mention command-fallback count
	if !strings.Contains(output, "command-fallback") {
		t.Error("output should contain command-fallback stats")
	}
}

func TestEmitWithTimestamp(t *testing.T) {
	tests, warnings := Translate(&GossFile{}, TranslateOptions{})
	ts := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	output, err := Emit(tests, warnings, EmitMeta{
		Source:    "goss.yaml",
		Timestamp: ts,
	})
	if err != nil {
		t.Fatalf("Emit() error = %v", err)
	}
	if !strings.Contains(output, "2024-01-15") {
		t.Errorf("output should contain timestamp date, got:\n%s", output)
	}
}

func TestEmitNoSourceNoTimestamp(t *testing.T) {
	tests, warnings := Translate(&GossFile{}, TranslateOptions{})
	output, err := Emit(tests, warnings, EmitMeta{})
	if err != nil {
		t.Fatalf("Emit() error = %v", err)
	}
	if output == "" {
		t.Error("Emit() returned empty string")
	}
}

func TestEmitStatsWithWarnings(t *testing.T) {
	warnings := []TranslationWarning{
		{GossKey: "service", Resource: "nginx", Category: WarnCommandFallback, Message: "systemctl fallback"},
		{GossKey: "dns", Resource: "example.com", Category: WarnCommandFallback, Message: "getent fallback"},
		{GossKey: "port", Resource: "tcp:notaport", Category: WarnSkipped, Message: "parse failed"},
	}
	result := EmitStats(warnings)
	if !strings.Contains(result, "Command fallbacks: 2") {
		t.Errorf("expected 'Command fallbacks: 2', got:\n%s", result)
	}
	if !strings.Contains(result, "Skipped (TODO):    1") {
		t.Errorf("expected 'Skipped (TODO):    1', got:\n%s", result)
	}
	// Should list individual warnings
	if !strings.Contains(result, "systemctl fallback") {
		t.Errorf("expected warning detail in output, got:\n%s", result)
	}
}

func TestEmitStatsNoWarnings(t *testing.T) {
	result := EmitStats(nil)
	if result == "" {
		t.Error("EmitStats(nil) returned empty string")
	}
	if !strings.Contains(result, "Command fallbacks: 0") {
		t.Errorf("expected zero counts, got:\n%s", result)
	}
	// Should NOT have a Warnings section
	if strings.Contains(result, "Warnings:") {
		t.Errorf("EmitStats(nil) should not include Warnings section, got:\n%s", result)
	}
}
