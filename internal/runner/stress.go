package runner

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// StressResult holds aggregate results from a stress test run.
type StressResult struct {
	TestName     string
	TotalRuns    int
	Passes       int
	Failures     int
	PassRate     float64
	Duration     time.Duration
	Concurrency  int
	Reliability  string // Stable, Flaky, Unreliable
	ErrorGroups  []ErrorGroup
	RunDurations []time.Duration
}

// ErrorGroup holds a deduplicated error with its occurrence count.
type ErrorGroup struct {
	Message string
	Count   int
}

// DedupErrors groups identical error messages and sorts by frequency (descending).
func DedupErrors(errors []string) []ErrorGroup {
	counts := make(map[string]int)
	for _, e := range errors {
		counts[e]++
	}
	groups := make([]ErrorGroup, 0, len(counts))
	for msg, cnt := range counts {
		groups = append(groups, ErrorGroup{Message: msg, Count: cnt})
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Count > groups[j].Count
	})
	return groups
}

// ReliabilityStatus returns a human-readable reliability label.
func ReliabilityStatus(passRate float64) string {
	switch {
	case passRate >= 100.0:
		return "Stable"
	case passRate >= 95.0:
		return "Flaky"
	default:
		return "Unreliable"
	}
}

// StressTest runs a single test N times with configurable concurrency.
func (r *Runner) StressTest(testName string, runs, workers int, failFast bool) StressResult {
	var target *schema.Test
	for i := range r.Config.Tests {
		if r.Config.Tests[i].Name == testName {
			target = &r.Config.Tests[i]
			break
		}
	}
	if target == nil {
		return StressResult{TestName: testName}
	}

	var (
		passes    atomic.Int64
		failures  atomic.Int64
		mu        sync.Mutex
		errors    []string
		durations []time.Duration
	)

	if len(r.Config.Lifecycle.BeforeAll) > 0 {
		RunLifecycleHooks(context.Background(), r.Config.Lifecycle.BeforeAll, nil, r.ConfigDir)
	}
	defer func() {
		if len(r.Config.Lifecycle.AfterAll) > 0 {
			RunLifecycleHooks(context.Background(), r.Config.Lifecycle.AfterAll, nil, r.ConfigDir)
		}
		CleanupBackgroundProcesses()
	}()

	start := time.Now()
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	for i := 0; i < runs; i++ {
		if failFast && failures.Load() > 0 {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			opts := RunOptions{}
			tr := r.runTestOnce(*target, opts)
			if tr.Passed {
				passes.Add(1)
			} else {
				failures.Add(1)
				errMsg := testErrorMessage(tr)
				if errMsg != "" {
					mu.Lock()
					errors = append(errors, errMsg)
					mu.Unlock()
				}
			}
			mu.Lock()
			durations = append(durations, tr.Duration)
			mu.Unlock()
		}()
	}
	wg.Wait()

	totalRuns := int(passes.Load() + failures.Load())
	var passRate float64
	if totalRuns > 0 {
		passRate = float64(passes.Load()) / float64(totalRuns) * 100.0
	}

	return StressResult{
		TestName:     testName,
		TotalRuns:    totalRuns,
		Passes:       int(passes.Load()),
		Failures:     int(failures.Load()),
		PassRate:     passRate,
		Duration:     time.Since(start),
		Concurrency:  workers,
		Reliability:  ReliabilityStatus(passRate),
		ErrorGroups:  DedupErrors(errors),
		RunDurations: durations,
	}
}

func testErrorMessage(tr TestResult) string {
	for _, a := range tr.Assertions {
		if !a.Passed {
			return fmt.Sprintf("%s: expected %s, got %s", a.Type, a.Expected, a.Actual)
		}
	}
	if tr.Error != nil {
		return tr.Error.Error()
	}
	return ""
}
