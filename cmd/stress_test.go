package cmd

import (
	"testing"
)

func TestStressCmd_Flags(t *testing.T) {
	if !stressCmd.HasFlags() {
		t.Fatal("stress command should have flags")
	}
	runs, err := stressCmd.Flags().GetInt("runs")
	if err != nil || runs != 50 {
		t.Errorf("expected --runs default 50, got %d, err %v", runs, err)
	}
	workers, err := stressCmd.Flags().GetInt("workers")
	if err != nil || workers != 1 {
		t.Errorf("expected --workers default 1, got %d, err %v", workers, err)
	}
}

func TestStressCmd_RequiresTestName(t *testing.T) {
	err := stressCmd.Args(stressCmd, []string{})
	if err == nil {
		t.Fatal("expected error when no test name provided")
	}
}
