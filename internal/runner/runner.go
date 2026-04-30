package runner

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/CosmoLabs-org/cosmo-smoke/internal/monorepo"
	"github.com/CosmoLabs-org/cosmo-smoke/internal/reporter"
	"github.com/CosmoLabs-org/cosmo-smoke/internal/schema"
)

// RunOptions controls test execution behavior.
type RunOptions struct {
	Tags        []string
	ExcludeTags []string
	FailFast    bool
	DryRun      bool
	Timeout     time.Duration // per-test override (0 = use config)
}

// SuiteResult holds the aggregate outcome.
type SuiteResult struct {
	Project         string
	Total           int
	Passed          int
	Failed          int
	Skipped         int
	AllowedFailures int
	Duration        time.Duration
	Tests           []TestResult
}

// TestResult holds one test's outcome.
type TestResult struct {
	Name           string
	Passed         bool
	Skipped        bool
	AllowedFailure bool // true if test failed but allow_failure was set
	Duration       time.Duration
	Assertions     []AssertionResult
	Error          error
	Attempts       int // number of attempts made (1 = no retry)
}

// Runner executes smoke tests from a config.
type Runner struct {
	Config        *schema.SmokeConfig
	Reporter      reporter.Reporter
	ConfigDir     string
	trace         *TraceContext
	TraceHealth   *TraceHealthTracker
	Vars          *VarStore
	lifecycleEnv  map[string]string
}

// Run executes all tests per the options and returns the suite result.
func (r *Runner) Run(opts RunOptions) (*SuiteResult, error) {
	start := time.Now()
	r.Vars = NewVarStore()
	r.lifecycleEnv = make(map[string]string)

	// Run before_all lifecycle hooks
	if len(r.Config.Lifecycle.BeforeAll) > 0 {
		env, err := RunLifecycleHooks(context.Background(), r.Config.Lifecycle.BeforeAll, r.lifecycleEnv, r.ConfigDir)
		r.lifecycleEnv = env
		if err != nil {
			return nil, fmt.Errorf("before_all lifecycle hook failed: %w", err)
		}
	}

	// Schedule after_all hooks to run regardless of test outcomes
	if len(r.Config.Lifecycle.AfterAll) > 0 {
		defer func() {
			RunLifecycleHooks(context.Background(), r.Config.Lifecycle.AfterAll, r.lifecycleEnv, r.ConfigDir)
		}()
	}

	// Run prerequisites
	if len(r.Config.Prereqs) > 0 {
		timeout := r.Config.Settings.Timeout.Duration
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		results := CheckPrerequisites(r.Config.Prereqs, timeout)
		for _, pr := range results {
			r.Reporter.PrereqStart(pr.Name)
			r.Reporter.PrereqResult(reporter.PrereqResultData{
				Name:   pr.Name,
				Passed: pr.Passed,
				Output: pr.Output,
				Hint:   pr.Hint,
				Error:  pr.Error,
			})
		}
		// Check for prereq failures
		for _, pr := range results {
			if !pr.Passed {
				return nil, fmt.Errorf("prerequisite %q failed: %v", pr.Name, pr.Error)
			}
		}
	}

	// Filter tests by tags
	tests := filterTests(r.Config.Tests, opts.Tags, opts.ExcludeTags)

	// Initialize trace context if otel is enabled
	if r.Config.OTel.Enabled {
		r.trace = NewTraceContext()
		if r.TraceHealth == nil {
			r.TraceHealth = NewTraceHealthTracker(10)
		}
	}

	suite := &SuiteResult{
		Project: r.Config.Project,
		Total:   len(tests),
	}

	failFast := opts.FailFast || r.Config.Settings.FailFast

	if r.Config.Settings.Parallel && !failFast {
		r.runParallel(tests, opts, suite)
	} else {
		r.runSequential(tests, opts, suite, failFast)
	}

	suite.Duration = time.Since(start)

	// Report summary
	summaryData := reporter.SuiteResultData{
		Project:         suite.Project,
		Total:           suite.Total,
		Passed:          suite.Passed,
		Failed:          suite.Failed,
		Skipped:         suite.Skipped,
		AllowedFailures: suite.AllowedFailures,
		Duration:        suite.Duration,
	}
	if r.TraceHealth != nil && r.TraceHealth.Total() > 0 {
		summaryData.TraceHealthPct = r.TraceHealth.HealthPct()
		summaryData.TraceDegraded = r.TraceHealth.Degraded(50.0)
	}
	r.Reporter.Summary(summaryData)

	return suite, nil
}

// RunMonorepo discovers and runs all sub-configs in a monorepo.
func (r *Runner) RunMonorepo(opts RunOptions, subConfigs []monorepo.SubConfig) (*SuiteResult, error) {
	start := time.Now()

	suite := &SuiteResult{
		Project: r.Config.Project,
	}

	for _, sc := range subConfigs {
		cfg, err := schema.Load(sc.Path)
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", sc.Path, err)
		}
		subRunner := &Runner{
			Config:    cfg,
			Reporter:  r.Reporter,
			ConfigDir: sc.Dir,
		}
		result, err := subRunner.Run(opts)
		if err != nil {
			return nil, fmt.Errorf("running %s: %w", sc.Project, err)
		}
		suite.Tests = append(suite.Tests, result.Tests...)
		suite.Passed += result.Passed
		suite.Failed += result.Failed
		suite.Skipped += result.Skipped
		suite.AllowedFailures += result.AllowedFailures
		suite.Total += result.Total
	}

	suite.Duration = time.Since(start)
	r.Reporter.Summary(reporter.SuiteResultData{
		Project:         suite.Project,
		Total:           suite.Total,
		Passed:          suite.Passed,
		Failed:          suite.Failed,
		Skipped:         suite.Skipped,
		AllowedFailures: suite.AllowedFailures,
		Duration:        suite.Duration,
	})
	return suite, nil
}

func (r *Runner) runSequential(tests []schema.Test, opts RunOptions, suite *SuiteResult, failFast bool) {
	stopped := false
	for _, t := range tests {
		if stopped {
			tr := TestResult{Name: t.Name, Skipped: true}
			suite.Tests = append(suite.Tests, tr)
			suite.Skipped++
			r.Reporter.TestStart(t.Name)
			r.Reporter.TestResult(reporter.TestResultData{Name: t.Name, Skipped: true})
			continue
		}

		tr := r.runTestWithHooks(t, opts)
		suite.Tests = append(suite.Tests, tr)
		if tr.Passed {
			suite.Passed++
		} else if tr.Skipped {
			suite.Skipped++
		} else if tr.AllowedFailure {
			suite.AllowedFailures++
		} else {
			suite.Failed++
			if failFast {
				stopped = true
			}
		}
	}
}

func (r *Runner) runParallel(tests []schema.Test, opts RunOptions, suite *SuiteResult) {
	results := make([]TestResult, len(tests))
	var wg sync.WaitGroup

	for i, t := range tests {
		wg.Add(1)
		go func(idx int, test schema.Test) {
			defer wg.Done()
			results[idx] = r.runTestWithHooks(test, opts)
		}(i, t)
	}
	wg.Wait()

	for _, tr := range results {
		suite.Tests = append(suite.Tests, tr)
		if tr.Passed {
			suite.Passed++
		} else if tr.Skipped {
			suite.Skipped++
		} else if tr.AllowedFailure {
			suite.AllowedFailures++
		} else {
			suite.Failed++
		}
	}
}

func (r *Runner) runTest(t schema.Test, opts RunOptions) TestResult {
	if t.Retry == nil || t.Retry.Count <= 1 {
		res := r.runTestOnce(t, opts)
		if res.Attempts == 0 {
			res.Attempts = 1
		}
		return res
	}
	var last TestResult
	backoff := t.Retry.Backoff.Duration
	for attempt := 1; attempt <= t.Retry.Count; attempt++ {
		last = r.runTestOnce(t, opts)
		if last.Passed {
			last.Attempts = attempt
			return last
		}
		if t.Retry.RetryOnTraceOnly && traceConfirmed(last.Assertions) {
			last.Attempts = attempt
			return last
		}
		if attempt < t.Retry.Count {
			time.Sleep(backoff)
			backoff *= 2
		}
	}
	last.Attempts = t.Retry.Count
	return last
}

func (r *Runner) runTestWithHooks(t schema.Test, opts RunOptions) TestResult {
	if len(r.Config.Lifecycle.BeforeEach) > 0 {
		env, err := RunLifecycleHooks(context.Background(), r.Config.Lifecycle.BeforeEach, r.lifecycleEnv, r.ConfigDir)
		r.lifecycleEnv = env
		if err != nil {
			return TestResult{
				Name:  t.Name,
				Error: fmt.Errorf("before_each lifecycle hook failed: %w", err),
			}
		}
	}

	result := r.runTest(t, opts)

	if len(r.Config.Lifecycle.AfterEach) > 0 {
		env, _ := RunLifecycleHooks(context.Background(), r.Config.Lifecycle.AfterEach, r.lifecycleEnv, r.ConfigDir)
		r.lifecycleEnv = env
	}

	return result
}

func (r *Runner) runTestOnce(t schema.Test, opts RunOptions) TestResult {
	r.Reporter.TestStart(t.Name)
	start := time.Now()

	// Evaluate skip_if conditions
	if t.SkipIf != nil && shouldSkip(t.SkipIf, r.ConfigDir) {
		tr := TestResult{Name: t.Name, Skipped: true, Duration: time.Since(start)}
		r.Reporter.TestResult(toReporterResult(tr))
		return tr
	}

	if opts.DryRun {
		tr := TestResult{Name: t.Name, Passed: true, Duration: time.Since(start)}
		r.Reporter.TestResult(toReporterResult(tr))
		return tr
	}

	// Create trace span for this test
	var span *SpanContext
	if r.trace != nil && r.trace.Enabled {
		span = r.trace.NewSpan()
	}

	// Determine timeout
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = t.Timeout.Duration
	}
	if timeout == 0 {
		timeout = r.Config.Settings.Timeout.Duration
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Run cleanup via defer
	if t.Cleanup != "" {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			c := exec.CommandContext(ctx, "sh", "-c", t.Cleanup)
			c.Dir = r.ConfigDir
			c.Run()
		}()
	}

	// Execute command (skip if no run command — standalone assertions only)
	var stdout, stderr bytes.Buffer
	exitCode := 0
	if t.Run != "" {
		runCmd := t.Run
		if r.Vars != nil {
			resolved, err := r.Vars.ResolveTemplate(t.Run)
			if err == nil {
				runCmd = resolved
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, "sh", "-c", runCmd)
		cmd.Dir = r.ConfigDir
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				tr := TestResult{
					Name:           t.Name,
					AllowedFailure: t.AllowFailure,
					Duration:       time.Since(start),
					Error:          err,
				}
				r.Reporter.TestResult(toReporterResult(tr))
				return tr
			}
		}
	}

	// Evaluate assertions
	var assertions []AssertionResult
	allPassed := true

	if t.Expect.ExitCode != nil {
		a := CheckExitCode(exitCode, *t.Expect.ExitCode)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}
	if t.Expect.StdoutContains != "" {
		a := CheckStdoutContains(stdout.String(), t.Expect.StdoutContains)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}
	if t.Expect.StdoutMatches != "" {
		a := CheckStdoutMatches(stdout.String(), t.Expect.StdoutMatches)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}
	if t.Expect.StderrContains != "" {
		a := CheckStderrContains(stderr.String(), t.Expect.StderrContains)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}
	if t.Expect.StderrMatches != "" {
		a := CheckStderrMatches(stderr.String(), t.Expect.StderrMatches)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}
	if t.Expect.EnvExists != "" {
		a := CheckEnvExists(t.Expect.EnvExists)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}
	if t.Expect.FileExists != "" {
		a := CheckFileExists(t.Expect.FileExists, r.ConfigDir)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}
		if t.Expect.FileSize != nil {
			a := CheckFileSize(t.Expect.FileSize, r.ConfigDir)
			assertions = append(assertions, a)
			if !a.Passed {
				allPassed = false
			}
		}
	if t.Expect.PortListening != nil {
		p := t.Expect.PortListening
		a := CheckPortListening(p.Port, p.Protocol, p.Host)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}
	if t.Expect.ProcessRunning != "" {
		a := CheckProcessRunning(t.Expect.ProcessRunning)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}
	if t.Expect.HTTP != nil {
		var httpResults []AssertionResult
		if span != nil && r.Config.OTel.TracePropagation {
			httpResults = CheckHTTPWithTrace(t.Expect.HTTP, span)
		} else {
			httpResults = CheckHTTP(t.Expect.HTTP)
		}
		for _, a := range httpResults {
			assertions = append(assertions, a)
			if !a.Passed {
				allPassed = false
			}
		}
	}
	if t.Expect.JSONField != nil {
		jsonResults := CheckJSONField(stdout.String(), t.Expect.JSONField)
		for _, a := range jsonResults {
			assertions = append(assertions, a)
			if !a.Passed {
				allPassed = false
			}
		}
	}
	if t.Expect.SSLCert != nil {
		a := CheckSSLCert(t.Expect.SSLCert)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}
	if t.Expect.Redis != nil {
		a := CheckRedisPing(t.Expect.Redis)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}
	if t.Expect.Memcached != nil {
		a := CheckMemcachedVersion(t.Expect.Memcached)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}
	if t.Expect.Postgres != nil {
		a := CheckPostgresPing(t.Expect.Postgres)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}
	if t.Expect.MySQL != nil {
		a := CheckMySQLPing(t.Expect.MySQL)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}
	if t.Expect.GRPCHealth != nil {
		var grpcResult AssertionResult
		if span != nil && r.Config.OTel.TracePropagation {
			grpcResult = CheckGRPCHealthWithTrace(t.Expect.GRPCHealth, span)
		} else {
			grpcResult = CheckGRPCHealth(t.Expect.GRPCHealth)
		}
		assertions = append(assertions, grpcResult)
		if !grpcResult.Passed {
			allPassed = false
		}
	}
	if t.Expect.DockerContainer != nil {
		a := CheckDockerContainerRunning(t.Expect.DockerContainer)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}
	if t.Expect.DockerImage != nil {
		a := CheckDockerImageExists(t.Expect.DockerImage)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}
	if t.Expect.URLReachable != nil {
		a := CheckURLReachable(t.Expect.URLReachable)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}
	if t.Expect.ServiceReachable != nil {
		a := CheckServiceReachable(t.Expect.ServiceReachable)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}
	if t.Expect.S3Bucket != nil {
		a := CheckS3Bucket(t.Expect.S3Bucket)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}
	if t.Expect.VersionCheck != nil {
		a := CheckVersion(t.Expect.VersionCheck)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}
	if t.Expect.WebSocket != nil {
		var wsResult AssertionResult
		if span != nil && r.Config.OTel.TracePropagation {
			wsResult = CheckWebSocketWithTrace(t.Expect.WebSocket, span)
		} else {
			wsResult = CheckWebSocket(t.Expect.WebSocket)
		}
		assertions = append(assertions, wsResult)
		if !wsResult.Passed {
			allPassed = false
		}
	}

	if t.Expect.OTelTrace != nil {
		check := t.Expect.OTelTrace
		if check.JaegerURL == "" {
			check.JaegerURL = r.Config.OTel.JaegerURL
		}
		if check.ServiceName == "" {
			check.ServiceName = r.Config.OTel.ServiceName
		}
		if check.ServiceName == "" {
			check.ServiceName = "smoke"
		}
		traceID := ""
		if r.trace != nil && r.trace.Enabled {
			traceID = r.trace.TraceID()
		}
		client := &http.Client{Timeout: check.Timeout.Duration + 5*time.Second}
		if client.Timeout <= 5*time.Second {
			client.Timeout = 10 * time.Second
		}
		a := CheckOTelTrace(check, traceID, client)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
		if r.TraceHealth != nil {
			r.TraceHealth.Record(a.Passed)
		}
	}

	if t.Expect.Credential != nil {
		a := CheckCredential(t.Expect.Credential, r.ConfigDir)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}

	if t.Expect.GraphQL != nil {
		graphqlResults := CheckGraphQL(t.Expect.GraphQL)
		for _, a := range graphqlResults {
			assertions = append(assertions, a)
			if !a.Passed {
				allPassed = false
			}
		}
	}

	if t.Expect.DeepLink != nil {
		dlResults := CheckDeepLink(t.Expect.DeepLink, r.ConfigDir)
		for _, a := range dlResults {
			assertions = append(assertions, a)
			if !a.Passed {
				allPassed = false
			}
		}
	}

	if t.Expect.DNS != nil {
		a := CheckDNS(t.Expect.DNS)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}

	if t.Expect.SMTP != nil {
		a := CheckSMTP(t.Expect.SMTP)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}

	if t.Expect.DockerCompose != nil {
		a := CheckDockerComposeHealthy(t.Expect.DockerCompose)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}

	if t.Expect.Ping != nil {
		a := CheckPing(t.Expect.Ping)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}

	if t.Expect.Mongo != nil {
		a := CheckMongoPing(t.Expect.Mongo)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}

	if t.Expect.Kafka != nil {
		a := CheckKafkaBroker(t.Expect.Kafka)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}

	if t.Expect.LDAP != nil {
		a := CheckLDAPBind(t.Expect.LDAP)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}

	if t.Expect.MQTT != nil {
		a := CheckMQTTPing(t.Expect.MQTT)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}

	if t.Expect.NTP != nil {
		a := CheckNTP(t.Expect.NTP)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}

	if t.Expect.K8sResource != nil {
		a := CheckK8sResource(t.Expect.K8sResource)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}

	duration := time.Since(start)

	if t.Expect.ResponseTimeMs != nil {
		a := CheckResponseTime(int(duration.Milliseconds()), *t.Expect.ResponseTimeMs)
		assertions = append(assertions, a)
		if !a.Passed {
			allPassed = false
		}
	}

	// Process extracts: store captured values for chained tests
	if allPassed && r.Vars != nil {
		processExtracts(&t.Expect, stdout.String(), r.Vars)
	}

	tr := TestResult{
		Name:           t.Name,
		Passed:         allPassed,
		AllowedFailure: !allPassed && t.AllowFailure,
		Duration:       duration,
		Assertions:     assertions,
	}
	r.Reporter.TestResult(toReporterResult(tr))
	return tr
}

func toReporterResult(tr TestResult) reporter.TestResultData {
	var assertions []reporter.AssertionDetail
	for _, a := range tr.Assertions {
		assertions = append(assertions, reporter.AssertionDetail{
			Type:     a.Type,
			Expected: a.Expected,
			Actual:   a.Actual,
			Passed:   a.Passed,
		})
	}
	return reporter.TestResultData{
		Name:           tr.Name,
		Passed:         tr.Passed,
		Skipped:        tr.Skipped,
		AllowedFailure: tr.AllowedFailure,
		Duration:       tr.Duration,
		Assertions:     assertions,
		Error:          tr.Error,
	}
}

func filterTests(tests []schema.Test, include, exclude []string) []schema.Test {
	if len(include) == 0 && len(exclude) == 0 {
		return tests
	}

	var filtered []schema.Test
	for _, t := range tests {
		if len(include) > 0 && !hasAnyTag(t.Tags, include) {
			continue
		}
		if len(exclude) > 0 && hasAnyTag(t.Tags, exclude) {
			continue
		}
		filtered = append(filtered, t)
	}
	return filtered
}

func hasAnyTag(tags, targets []string) bool {
	for _, tag := range tags {
		for _, target := range targets {
			if strings.EqualFold(tag, target) {
				return true
			}
		}
	}
	return false
}

// shouldSkip evaluates skip_if conditions and returns true if the test should be skipped.
func shouldSkip(si *schema.SkipIf, configDir string) bool {
	if si == nil {
		return false
	}
	if si.EnvUnset != "" {
		if _, ok := os.LookupEnv(si.EnvUnset); !ok {
			return true
		}
	}
	if si.EnvEquals != nil {
		if val, ok := os.LookupEnv(si.EnvEquals.Var); ok && val == si.EnvEquals.Value {
			return true
		}
	}
	if si.FileMissing != "" {
		path := si.FileMissing
		if !strings.HasPrefix(path, "/") {
			path = configDir + "/" + path
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return true
		}
	}
	return false
}

// traceConfirmed checks whether any otel_trace assertion in the results passed,
// meaning the trace was received at the collector.
func traceConfirmed(assertions []AssertionResult) bool {
	for _, a := range assertions {
		if a.Type == "otel_trace" && a.Passed {
			return true
		}
	}
	return false
}
