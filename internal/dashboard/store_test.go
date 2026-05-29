package dashboard

import (
	"encoding/json"
	"testing"
)

func testStore(t testing.TB, maxRuns ...int) *Store {
	t.Helper()
	mr := 100
	if len(maxRuns) > 0 {
		mr = maxRuns[0]
	}
	s, err := NewStore(":memory:", mr)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func makePayload(project string, total, passed, failed int, durationMs int64) []byte {
	data := map[string]interface{}{
		"project":     project,
		"total":       total,
		"passed":      passed,
		"failed":      failed,
		"skipped":     0,
		"duration_ms": durationMs,
	}
	b, _ := json.Marshal(data)
	return b
}

func TestStore_InsertAndGetProjects(t *testing.T) {
	s := testStore(t)

	s.InsertRun("cosmo-api", makePayload("cosmo-api", 10, 10, 0, 3400))
	s.InsertRun("cosmo-web", makePayload("cosmo-web", 8, 6, 2, 2100))

	projects, err := s.GetProjects()
	if err != nil {
		t.Fatalf("GetProjects: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("projects = %d, want 2", len(projects))
	}

	api := projects[0]
	if api.Name != "cosmo-api" {
		t.Errorf("project name = %q, want cosmo-api", api.Name)
	}
	if api.LatestStatus != "healthy" {
		t.Errorf("status = %q, want healthy", api.LatestStatus)
	}
	if api.TotalTests != 10 {
		t.Errorf("total = %d, want 10", api.TotalTests)
	}

	web := projects[1]
	if web.LatestStatus != "failing" {
		t.Errorf("status = %q, want failing", web.LatestStatus)
	}
}

func TestStore_GetProjectHistory(t *testing.T) {
	s := testStore(t)

	for i := 0; i < 3; i++ {
		s.InsertRun("cosmo-api", makePayload("cosmo-api", 10, 10, 0, 1000))
	}

	runs, err := s.GetProjectHistory("cosmo-api", 10)
	if err != nil {
		t.Fatalf("GetProjectHistory: %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("runs = %d, want 3", len(runs))
	}
	if runs[0].Project != "cosmo-api" {
		t.Errorf("project = %q, want cosmo-api", runs[0].Project)
	}
	if runs[0].Timestamp.Before(runs[2].Timestamp) {
		t.Error("expected descending order by timestamp")
	}
}

func TestStore_HistoryLimit(t *testing.T) {
	s := testStore(t)

	for i := 0; i < 10; i++ {
		s.InsertRun("cosmo-api", makePayload("cosmo-api", 10, 10, 0, 1000))
	}

	runs, err := s.GetProjectHistory("cosmo-api", 5)
	if err != nil {
		t.Fatalf("GetProjectHistory: %v", err)
	}
	if len(runs) != 5 {
		t.Errorf("runs = %d, want 5 (limit)", len(runs))
	}
}

func TestStore_PruneOldRuns(t *testing.T) {
	s := testStore(t, 3)

	for i := 0; i < 5; i++ {
		s.InsertRun("cosmo-api", makePayload("cosmo-api", 10, 10, 0, 1000))
	}

	runs, _ := s.GetProjectHistory("cosmo-api", 100)
	if len(runs) != 3 {
		t.Errorf("runs after prune = %d, want 3", len(runs))
	}
}

func TestStore_EmptyProjects(t *testing.T) {
	s := testStore(t)

	projects, err := s.GetProjects()
	if err != nil {
		t.Fatalf("GetProjects: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("projects = %d, want 0", len(projects))
	}
}

func TestStore_NonexistentProject(t *testing.T) {
	s := testStore(t)

	runs, err := s.GetProjectHistory("nonexistent", 10)
	if err != nil {
		t.Fatalf("GetProjectHistory: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("runs = %d, want 0", len(runs))
	}
}

func TestStore_InvalidPayload(t *testing.T) {
	s := testStore(t)

	_, err := s.InsertRun("bad", []byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON payload")
	}
}

func TestStore_AgeSeconds(t *testing.T) {
	s := testStore(t)
	s.InsertRun("cosmo-api", makePayload("cosmo-api", 5, 5, 0, 500))

	projects, _ := s.GetProjects()
	if len(projects) != 1 {
		t.Fatal("expected 1 project")
	}
	if projects[0].LastRunAgeSeconds > 2 {
		t.Errorf("age = %d, expected < 2", projects[0].LastRunAgeSeconds)
	}
}

// TestNewStore_MaxRunsDefault verifies that maxRunsPerProject <= 0 defaults to 1000.
func TestNewStore_MaxRunsDefault(t *testing.T) {
	s, err := NewStore(":memory:", 0)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer s.Close()

	if s.maxRunsPerProject != 1000 {
		t.Errorf("maxRunsPerProject = %d, want 1000", s.maxRunsPerProject)
	}
}

// TestNewStore_NegativeMaxRuns verifies that negative maxRunsPerProject also defaults to 1000.
func TestNewStore_NegativeMaxRuns(t *testing.T) {
	s, err := NewStore(":memory:", -5)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer s.Close()

	if s.maxRunsPerProject != 1000 {
		t.Errorf("maxRunsPerProject = %d, want 1000", s.maxRunsPerProject)
	}
}

// TestGetProjectHistory_LimitZeroDefaults verifies that limit=0 defaults to 50.
func TestGetProjectHistory_LimitZeroDefaults(t *testing.T) {
	s := testStore(t)

	// Insert 60 runs so we can verify the default limit of 50 kicks in.
	for i := 0; i < 60; i++ {
		s.InsertRun("cosmo-api", makePayload("cosmo-api", 10, 10, 0, 1000))
	}

	runs, err := s.GetProjectHistory("cosmo-api", 0)
	if err != nil {
		t.Fatalf("GetProjectHistory: %v", err)
	}
	if len(runs) != 50 {
		t.Errorf("runs = %d, want 50 (default limit when limit=0)", len(runs))
	}
}

// TestGetProjectHistory_NegativeLimitDefaults verifies that limit<0 also defaults to 50.
func TestGetProjectHistory_NegativeLimitDefaults(t *testing.T) {
	s := testStore(t)

	for i := 0; i < 60; i++ {
		s.InsertRun("cosmo-api", makePayload("cosmo-api", 10, 10, 0, 1000))
	}

	runs, err := s.GetProjectHistory("cosmo-api", -1)
	if err != nil {
		t.Fatalf("GetProjectHistory: %v", err)
	}
	if len(runs) != 50 {
		t.Errorf("runs = %d, want 50 (default limit when limit<0)", len(runs))
	}
}

// TestGetProjects_MultipleStatuses verifies healthy/failing classification across many projects.
func TestGetProjects_MultipleStatuses(t *testing.T) {
	s := testStore(t)

	// Three healthy projects, two failing.
	for _, name := range []string{"proj-a", "proj-b", "proj-c"} {
		s.InsertRun(name, makePayload(name, 10, 10, 0, 500))
	}
	for _, name := range []string{"proj-d", "proj-e"} {
		s.InsertRun(name, makePayload(name, 10, 7, 3, 500))
	}

	projects, err := s.GetProjects()
	if err != nil {
		t.Fatalf("GetProjects: %v", err)
	}
	if len(projects) != 5 {
		t.Fatalf("projects = %d, want 5", len(projects))
	}

	healthy, failing := 0, 0
	for _, p := range projects {
		switch p.LatestStatus {
		case "healthy":
			healthy++
		case "failing":
			failing++
		}
	}
	if healthy != 3 {
		t.Errorf("healthy = %d, want 3", healthy)
	}
	if failing != 2 {
		t.Errorf("failing = %d, want 2", failing)
	}
}

// TestInsertRun_ExceedsLimit verifies that INSERT beyond maxRuns triggers the DELETE pruning path.
func TestInsertRun_ExceedsLimit(t *testing.T) {
	s := testStore(t, 2)

	// Insert 3 runs — third one must trigger pruning.
	for i := 0; i < 3; i++ {
		if _, err := s.InsertRun("proj", makePayload("proj", 5, 5, 0, 100)); err != nil {
			t.Fatalf("InsertRun[%d]: %v", i, err)
		}
	}

	runs, err := s.GetProjectHistory("proj", 100)
	if err != nil {
		t.Fatalf("GetProjectHistory: %v", err)
	}
	if len(runs) != 2 {
		t.Errorf("runs after prune = %d, want 2", len(runs))
	}
}

// TestStore_ClosedDB_InsertRun verifies that InsertRun returns an error on a closed DB.
func TestStore_ClosedDB_InsertRun(t *testing.T) {
	s, err := NewStore(":memory:", 10)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	s.db.Close() // Close underlying DB before use.

	_, err = s.InsertRun("proj", makePayload("proj", 5, 5, 0, 100))
	if err == nil {
		t.Error("expected error from InsertRun on closed DB")
	}
}

// TestStore_ClosedDB_GetProjects verifies that GetProjects returns an error on a closed DB.
func TestStore_ClosedDB_GetProjects(t *testing.T) {
	s, err := NewStore(":memory:", 10)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	s.db.Close()

	_, err = s.GetProjects()
	if err == nil {
		t.Error("expected error from GetProjects on closed DB")
	}
}

// TestStore_ClosedDB_GetProjectHistory verifies that GetProjectHistory returns an error on a closed DB.
func TestStore_ClosedDB_GetProjectHistory(t *testing.T) {
	s, err := NewStore(":memory:", 10)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	s.db.Close()

	_, err = s.GetProjectHistory("proj", 10)
	if err == nil {
		t.Error("expected error from GetProjectHistory on closed DB")
	}
}

// TestNewStore_MigrateError verifies that NewStore returns an error when migrate fails.
// modernc sqlite: sql.Open is lazy (always succeeds). The migrate Exec fails when the
// path points to a directory (not a file), which SQLite cannot open as a database.
func TestNewStore_MigrateError(t *testing.T) {
	// A directory path is accepted by sql.Open but rejected by the first Exec (migrate).
	dir := t.TempDir()
	_, err := NewStore(dir, 10) // dir itself is not a valid SQLite file
	if err == nil {
		t.Error("expected NewStore to fail when path is a directory (migrate should fail)")
	}
}
