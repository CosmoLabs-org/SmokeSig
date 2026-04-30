package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CosmoLabs-org/cosmo-smoke/internal/schema"
)

func TestCheckFileSize(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files of known sizes
	smallFile := filepath.Join(tmpDir, "small.txt")
	os.WriteFile(smallFile, make([]byte, 100), 0644)

	largeFile := filepath.Join(tmpDir, "large.bin")
	os.WriteFile(largeFile, make([]byte, 5*1024*1024), 0644) // 5MB

	tests := []struct {
		name      string
		check     *schema.FileSizeCheck
		configDir string
		wantPass  bool
		wantType  string
	}{
		{
			name:     "file within range",
			check:    &schema.FileSizeCheck{Path: smallFile, MinBytes: int64Ptr(50), MaxBytes: int64Ptr(200)},
			wantPass: true,
			wantType: "file_size",
		},
		{
			name:     "file exceeds max_bytes",
			check:    &schema.FileSizeCheck{Path: largeFile, MaxBytes: int64Ptr(1000)},
			wantPass: false,
			wantType: "file_size",
		},
		{
			name:     "file below min_bytes",
			check:    &schema.FileSizeCheck{Path: smallFile, MinBytes: int64Ptr(500)},
			wantPass: false,
			wantType: "file_size",
		},
		{
			name:     "file not found",
			check:    &schema.FileSizeCheck{Path: filepath.Join(tmpDir, "nonexistent.txt")},
			wantPass: false,
			wantType: "file_size",
		},
		{
			name:     "max_bytes only",
			check:    &schema.FileSizeCheck{Path: smallFile, MaxBytes: int64Ptr(1000)},
			wantPass: true,
			wantType: "file_size",
		},
		{
			name:     "min_bytes only",
			check:    &schema.FileSizeCheck{Path: smallFile, MinBytes: int64Ptr(50)},
			wantPass: true,
			wantType: "file_size",
		},
		{
			name:     "no min or max just checks existence",
			check:    &schema.FileSizeCheck{Path: smallFile},
			wantPass: true,
			wantType: "file_size",
		},
		{
			name:     "no min or max file missing",
			check:    &schema.FileSizeCheck{Path: filepath.Join(tmpDir, "missing")},
			wantPass: false,
			wantType: "file_size",
		},
		{
			name:      "relative path resolved against configDir",
			check:     &schema.FileSizeCheck{Path: "small.txt", MinBytes: int64Ptr(50), MaxBytes: int64Ptr(200)},
			configDir: tmpDir,
			wantPass:  true,
			wantType:  "file_size",
		},
		{
			name:     "exact size boundary passes max",
			check:    &schema.FileSizeCheck{Path: smallFile, MaxBytes: int64Ptr(100)},
			wantPass: true,
			wantType: "file_size",
		},
		{
			name:     "exact size boundary passes min",
			check:    &schema.FileSizeCheck{Path: smallFile, MinBytes: int64Ptr(100)},
			wantPass: true,
			wantType: "file_size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configDir := tt.configDir
			if configDir == "" {
				configDir = tmpDir
			}
			got := CheckFileSize(tt.check, configDir)
			if got.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tt.wantType)
			}
			if got.Passed != tt.wantPass {
				t.Errorf("Passed = %v, want %v\n  Expected: %s\n  Actual: %s",
					got.Passed, tt.wantPass, got.Expected, got.Actual)
			}
		})
	}
}

func int64Ptr(v int64) *int64 { return &v }
