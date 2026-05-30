package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTempGoss creates a temp Goss YAML file and returns the path.
func writeTempGoss(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "goss.yaml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func resetMigrateVars(t *testing.T) {
	t.Helper()
	origDistro := migrateDistro
	origOutput := migrateOutput
	origOverwrite := migrateOverwrite
	origStrict := migrateStrict
	origStats := migrateStats
	t.Cleanup(func() {
		migrateDistro = origDistro
		migrateOutput = origOutput
		migrateOverwrite = origOverwrite
		migrateStrict = origStrict
		migrateStats = origStats
	})
	migrateDistro = "deb"
	migrateOutput = ""
	migrateOverwrite = false
	migrateStrict = false
	migrateStats = false
}

// minimalGoss is a valid Goss YAML with a port assertion.
const minimalGoss = `port:
  tcp:8080:
    listening: true
`

// TestRunMigrateGoss_Success verifies a valid Goss file translates without error.
func TestRunMigrateGoss_Success(t *testing.T) {
	resetMigrateVars(t)
	p := writeTempGoss(t, minimalGoss)

	if err := runMigrateGoss(nil, []string{p}); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// TestRunMigrateGoss_InvalidDistro verifies unsupported distro returns an error.
func TestRunMigrateGoss_InvalidDistro(t *testing.T) {
	resetMigrateVars(t)
	migrateDistro = "arch"
	p := writeTempGoss(t, minimalGoss)

	err := runMigrateGoss(nil, []string{p})
	if err == nil {
		t.Fatal("expected error for invalid distro, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported distro") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestRunMigrateGoss_MissingFile verifies a missing input file returns an error.
func TestRunMigrateGoss_MissingFile(t *testing.T) {
	resetMigrateVars(t)
	err := runMigrateGoss(nil, []string{"/nonexistent/path/goss.yaml"})
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// TestRunMigrateGoss_InvalidYAML verifies garbage YAML returns an error.
func TestRunMigrateGoss_InvalidYAML(t *testing.T) {
	resetMigrateVars(t)
	p := writeTempGoss(t, "{{{{not valid yaml at all}}}}")
	err := runMigrateGoss(nil, []string{p})
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

// TestRunMigrateGoss_OutputFile verifies writing to a file works.
func TestRunMigrateGoss_OutputFile(t *testing.T) {
	resetMigrateVars(t)
	p := writeTempGoss(t, minimalGoss)
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "out.yaml")
	migrateOutput = outPath

	if err := runMigrateGoss(nil, []string{p}); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Errorf("expected output file to exist: %v", err)
	}
}

// TestRunMigrateGoss_OutputFileExists verifies overwrite protection without --overwrite.
func TestRunMigrateGoss_OutputFileExists(t *testing.T) {
	resetMigrateVars(t)
	p := writeTempGoss(t, minimalGoss)
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "out.yaml")
	// Pre-create the file.
	if err := os.WriteFile(outPath, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	migrateOutput = outPath
	migrateOverwrite = false

	err := runMigrateGoss(nil, []string{p})
	if err == nil {
		t.Fatal("expected error when output file already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestRunMigrateGoss_OverwriteFlag verifies --overwrite replaces an existing file.
func TestRunMigrateGoss_OverwriteFlag(t *testing.T) {
	resetMigrateVars(t)
	p := writeTempGoss(t, minimalGoss)
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "out.yaml")
	if err := os.WriteFile(outPath, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	migrateOutput = outPath
	migrateOverwrite = true

	if err := runMigrateGoss(nil, []string{p}); err != nil {
		t.Fatalf("expected nil with --overwrite, got %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "old" {
		t.Error("expected file to be overwritten, but content is unchanged")
	}
}

// TestRunMigrateGoss_ValidDistros verifies all supported distros are accepted.
func TestRunMigrateGoss_ValidDistros(t *testing.T) {
	p := writeTempGoss(t, minimalGoss)
	for _, d := range []string{"deb", "rpm", "apk"} {
		t.Run(d, func(t *testing.T) {
			resetMigrateVars(t)
			migrateDistro = d
			if err := runMigrateGoss(nil, []string{p}); err != nil {
				t.Errorf("distro %q: unexpected error: %v", d, err)
			}
		})
	}
}

// TestRunMigrateGoss_Stats verifies --stats doesn't crash.
func TestRunMigrateGoss_Stats(t *testing.T) {
	resetMigrateVars(t)
	migrateStats = true
	p := writeTempGoss(t, minimalGoss)
	if err := runMigrateGoss(nil, []string{p}); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// TestMigrateCommand_Registered verifies the migrate sub-command is on root.
func TestMigrateCommand_Registered(t *testing.T) {
	var found bool
	for _, sub := range rootCmd.Commands() {
		if sub.Use == "migrate" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("migrate command not registered on rootCmd")
	}
}
