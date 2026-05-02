---
branch: mobile-deep-link-assertion
created: "2026-04-21"
goals_completed: 0
goals_total: 39
origin: migrated by ccs prompts migrate
priority: medium
related_prompts: []
requires_reading: []
schema_version: 1
status: PENDING
tags: []
title: Mobile Deep Link Assertion — Implementation Plan
---

# Mobile Deep Link Assertion — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a two-tier progressive `deep_link` assertion type that validates mobile deep link configuration and resolution for Android, iOS, React Native, and Flutter projects.

**Architecture:** Single `deep_link` assertion field on `schema.Expect` with auto-tier detection. Tier 1 uses HTTP checks (`assetlinks.json`, `apple-app-site-association`) and local config file parsing. Tier 2 uses `adb`/`xcrun` when available. Four new detector types for mobile project auto-detection.

**Tech Stack:** Go stdlib only (`net/http`, `encoding/json`, `encoding/xml`, `os/exec`). No new external dependencies.

**Design spec:** `docs/brainstorming/2026-04-21-mobile-deep-link-assertion.md`

---

## File Structure

| Action | File | Responsibility |
|--------|------|---------------|
| Create | `internal/runner/assertion_deeplink.go` | Tier 1 + Tier 2 assertion logic |
| Create | `internal/runner/assertion_deeplink_test.go` | Unit tests with mock HTTP server |
| Modify | `internal/schema/schema.go` | `DeepLink` struct on `Expect` |
| Modify | `internal/detector/detector.go` | 4 mobile project types |
| Modify | `internal/detector/templates.go` | Mobile project `smoke init` templates |
| Modify | `internal/runner/runner.go` | Wire `deep_link` into `runTestOnce` |

---

## Chunk 1: Schema + Detector Types

### Task 1: Add DeepLink struct to schema

**Files:**
- Modify: `internal/schema/schema.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/schema/schema_test.go` (or create schema_decode_test.go if needed):

```go
func TestDeepLinkAssertionParsing(t *testing.T) {
    yaml := `
version: 1
tests:
  - name: deep link test
    run: "true"
    expect:
      deep_link:
        url: "myapp://product/123"
        android_package: "com.myapp"
        ios_bundle_id: "com.myapp"
        tier: auto
`
    cfg, err := schema.Parse([]byte(yaml))
    if err != nil {
        t.Fatal(err)
    }
    if len(cfg.Tests) != 1 {
        t.Fatalf("expected 1 test, got %d", len(cfg.Tests))
    }
    dl := cfg.Tests[0].Expect.DeepLink
    if dl == nil {
        t.Fatal("expected deep_link to be parsed")
    }
    if dl.URL != "myapp://product/123" {
        t.Errorf("url = %q, want myapp://product/123", dl.URL)
    }
    if dl.AndroidPackage != "com.myapp" {
        t.Errorf("android_package = %q", dl.AndroidPackage)
    }
    if dl.IOSBundleID != "com.myapp" {
        t.Errorf("ios_bundle_id = %q", dl.IOSBundleID)
    }
    if dl.Tier != "auto" {
        t.Errorf("tier = %q", dl.Tier)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/schema/ -run TestDeepLinkAssertionParsing -v`
Expected: FAIL — `DeepLink` field doesn't exist

- [ ] **Step 3: Add DeepLink struct to Expect**

In `internal/schema/schema.go`, add the struct and field:

```go
type DeepLinkCheck struct {
    URL                 string   `yaml:"url" json:"url"`
    AndroidPackage      string   `yaml:"android_package,omitempty" json:"android_package,omitempty"`
    IOSBundleID         string   `yaml:"ios_bundle_id,omitempty" json:"ios_bundle_id,omitempty"`
    IOSAssociatedDomains []string `yaml:"ios_associated_domains,omitempty" json:"ios_associated_domains,omitempty"`
    CheckAssetlinks     *bool    `yaml:"check_assetlinks,omitempty" json:"check_assetlinks,omitempty"`
    CheckAASA           *bool    `yaml:"check_aasa,omitempty" json:"check_aasa,omitempty"`
    Tier                string   `yaml:"tier,omitempty" json:"tier,omitempty"`
}
```

Add `DeepLink *DeepLinkCheck` to the `Expect` struct (alongside existing assertion fields).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/schema/ -run TestDeepLinkAssertionParsing -v`
Expected: PASS

- [ ] **Step 5: Run full schema tests**

Run: `go test ./internal/schema/ -v`
Expected: All pass (no regressions)

- [ ] **Step 6: Commit**

```bash
git add internal/schema/schema.go internal/schema/schema_test.go
git commit -m "feat(schema): add DeepLink assertion struct for mobile deep link checks"
```

---

### Task 2: Add mobile project types to detector

**Files:**
- Modify: `internal/detector/detector.go`
- Modify: `internal/detector/detector_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestDetect_ReactNative(t *testing.T) {
    dir := t.TempDir()
    os.WriteFile(filepath.Join(dir, "app.json"), []byte(`{"name":"MyApp"}`), 0644)
    os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"react-native":"^0.72"}}`), 0644)
    types := Detect(dir)
    found := false
    for _, tp := range types {
        if tp == ReactNative { found = true }
    }
    if !found { t.Error("expected ReactNative type detected") }
}

func TestDetect_Flutter(t *testing.T) {
    dir := t.TempDir()
    os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte(`name: myapp\ndependencies:\n  flutter:\n    sdk: flutter`), 0644)
    types := Detect(dir)
    found := false
    for _, tp := range types {
        if tp == Flutter { found = true }
    }
    if !found { t.Error("expected Flutter type detected") }
}

func TestDetect_IOS(t *testing.T) {
    dir := t.TempDir()
    os.Mkdir(filepath.Join(dir, "MyApp.xcodeproj"), 0755)
    types := Detect(dir)
    found := false
    for _, tp := range types {
        if tp == IOS { found = true }
    }
    if !found { t.Error("expected IOS type detected") }
}

func TestDetect_Android(t *testing.T) {
    dir := t.TempDir()
    os.WriteFile(filepath.Join(dir, "build.gradle"), []byte("apply plugin: 'com.android.application'"), 0644)
    types := Detect(dir)
    found := false
    for _, tp := range types {
        if tp == Android { found = true }
    }
    if !found { t.Error("expected Android type detected") }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/detector/ -run "TestDetect_ReactNative|TestDetect_Flutter|TestDetect_IOS|TestDetect_Android" -v`
Expected: FAIL — constants don't exist

- [ ] **Step 3: Add constants and detection logic**

In `internal/detector/detector.go`, add to the const block:

```go
ReactNative ProjectType = "react-native"
Flutter     ProjectType = "flutter"
IOS         ProjectType = "ios"
Android     ProjectType = "android"
```

Add detection rules in `Detect()` after Rust check:

```go
// React Native: app.json + react-native dependency
if exists(dir, "app.json") {
    if hasDepInPackageJSON(dir, "react-native") || exists(dir, "metro.config.js") {
        types = append(types, ReactNative)
    }
}
// Flutter: pubspec.yaml with flutter dependency
if exists(dir, "pubspec.yaml") {
    if hasFlutterDep(dir) {
        types = append(types, Flutter)
    }
}
// iOS native: xcodeproj/xcworkspace or Podfile (skip if already detected as RN/Flutter)
if !hasType(types, ReactNative) && !hasType(types, Flutter) {
    if hasGlob(dir, "*.xcodeproj") || hasGlob(dir, "*.xcworkspace") || exists(dir, "Podfile") {
        types = append(types, IOS)
    }
}
// Android native: build.gradle without Go/Node (skip if already RN/Flutter)
if !hasType(types, ReactNative) && !hasType(types, Flutter) {
    if exists(dir, "build.gradle") || exists(dir, "build.gradle.kts") {
        if !exists(dir, "go.mod") && !exists(dir, "package.json") {
            types = append(types, Android)
        }
    }
}
```

Add helper functions `hasDepInPackageJSON`, `hasFlutterDep`, `hasType`, `hasGlob`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/detector/ -run "TestDetect_ReactNative|TestDetect_Flutter|TestDetect_IOS|TestDetect_Android" -v`
Expected: PASS

- [ ] **Step 5: Run full detector tests**

Run: `go test ./internal/detector/ -v`
Expected: All pass

- [ ] **Step 6: Commit**

```bash
git add internal/detector/detector.go internal/detector/detector_test.go
git commit -m "feat(detector): add ReactNative, Flutter, iOS, Android project types"
```

---

## Chunk 2: Tier 1 Assertion — HTTP Checks

### Task 3: Implement assetlinks.json and AASA validation

**Files:**
- Create: `internal/runner/assertion_deeplink.go`
- Create: `internal/runner/assertion_deeplink_test.go`

- [ ] **Step 1: Write failing tests for assetlinks.json check**

```go
func TestCheckAssetlinks_Valid(t *testing.T) {
    // Serve valid assetlinks.json from test HTTP server
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/.well-known/assetlinks.json" {
            w.WriteHeader(200)
            fmt.Fprintln(w, `[{"relation":["delegate_permission/common.handle_all_urls"],"target":{"namespace":"android_app","package_name":"com.myapp"}}]`)
            return
        }
        w.WriteHeader(404)
    }))
    defer srv.Close()

    result := CheckAssetlinks(srv.URL, "com.myapp")
    if !result.Passed {
        t.Errorf("expected pass, got: %s", result.Actual)
    }
}

func TestCheckAssetlinks_MissingPackage(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(200)
        fmt.Fprintln(w, `[{"relation":["delegate_permission/common.handle_all_urls"],"target":{"namespace":"android_app","package_name":"com.other"}}]`)
    }))
    defer srv.Close()

    result := CheckAssetlinks(srv.URL, "com.myapp")
    if result.Passed {
        t.Error("expected failure for mismatched package")
    }
}

func TestCheckAASA_Valid(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if strings.Contains(r.URL.Path, "apple-app-site-association") {
            w.WriteHeader(200)
            fmt.Fprintln(w, `{"applinks":{"details":[{"appIDs":["com.myapp"],"components":[{"/*":{}}]}]}}`)
            return
        }
        w.WriteHeader(404)
    }))
    defer srv.Close()

    result := CheckAASA(srv.URL, "com.myapp")
    if !result.Passed {
        t.Errorf("expected pass, got: %s", result.Actual)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runner/ -run "TestCheckAssetlinks|TestCheckAASA" -v`
Expected: FAIL — functions don't exist

- [ ] **Step 3: Implement CheckAssetlinks and CheckAASA**

In `internal/runner/assertion_deeplink.go`:

```go
package runner

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "time"
)

type assetlinkStatement struct {
    Relation []string `json:"relation"`
    Target   struct {
        Namespace   string `json:"namespace"`
        PackageName string `json:"package_name"`
    } `json:"target"`
}

func CheckAssetlinks(baseURL, expectedPackage string) AssertionResult {
    url := strings.TrimRight(baseURL, "/") + "/.well-known/assetlinks.json"
    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Get(url)
    if err != nil {
        return AssertionResult{Type: "deep_link.assetlinks", Passed: false, Expected: "assetlinks.json accessible", Actual: fmt.Sprintf("HTTP error: %v", err)}
    }
    defer resp.Body.Close()
    if resp.StatusCode != 200 {
        return AssertionResult{Type: "deep_link.assetlinks", Passed: false, Expected: "HTTP 200", Actual: fmt.Sprintf("HTTP %d", resp.StatusCode)}
    }
    var statements []assetlinkStatement
    if err := json.NewDecoder(resp.Body).Decode(&statements); err != nil {
        return AssertionResult{Type: "deep_link.assetlinks", Passed: false, Expected: "valid JSON", Actual: fmt.Sprintf("parse error: %v", err)}
    }
    for _, s := range statements {
        hasRelation := false
        for _, r := range s.Relation {
            if r == "delegate_permission/common.handle_all_urls" { hasRelation = true; break }
        }
        if hasRelation && s.Target.Namespace == "android_app" {
            if expectedPackage == "" || s.Target.PackageName == expectedPackage {
                return AssertionResult{Type: "deep_link.assetlinks", Passed: true}
            }
        }
    }
    return AssertionResult{Type: "deep_link.assetlinks", Passed: false, Expected: fmt.Sprintf("package %q in assetlinks.json", expectedPackage), Actual: "no matching statement found"}
}

func CheckAASA(baseURL, expectedBundleID string) AssertionResult {
    // Try both standard paths
    for _, path := range []string{"/apple-app-site-association", "/.well-known/apple-app-site-association"} {
        url := strings.TrimRight(baseURL, "/") + path
        client := &http.Client{Timeout: 10 * time.Second}
        resp, err := client.Get(url)
        if err != nil || resp.StatusCode != 200 {
            resp.Body.Close()
            continue
        }
        var aasa struct {
            Applinks struct {
                Details []struct {
                    AppIDs []string `json:"appIDs"`
                } `json:"details"`
            } `json:"applinks"`
        }
        err = json.NewDecoder(resp.Body).Decode(&aasa)
        resp.Body.Close()
        if err != nil { continue }
        for _, d := range aasa.Applinks.Details {
            for _, id := range d.AppIDs {
                if expectedBundleID == "" || strings.HasSuffix(id, expectedBundleID) {
                    return AssertionResult{Type: "deep_link.aasa", Passed: true}
                }
            }
        }
    }
    return AssertionResult{Type: "deep_link.aasa", Passed: false, Expected: "valid AASA with matching bundle ID", Actual: "no matching apple-app-site-association found"}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/runner/ -run "TestCheckAssetlinks|TestCheckAASA" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/runner/assertion_deeplink.go internal/runner/assertion_deeplink_test.go
git commit -m "feat(runner): add assetlinks.json and AASA HTTP validation for deep link tier 1"
```

---

## Chunk 3: Tier 1 — Local Config + Tier 2 Resolution

### Task 4: Local config file checks and adb/xcrun resolution

**Files:**
- Modify: `internal/runner/assertion_deeplink.go`
- Modify: `internal/runner/assertion_deeplink_test.go`

- [ ] **Step 1: Write failing tests for CheckDeepLink (main entry point)**

Test that `CheckDeepLink` routes to tier 1 checks for HTTP URLs, checks local config for custom schemes, and optionally calls tier 2.

```go
func TestCheckDeepLink_HTTPSChecksAssetlinks(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/.well-known/assetlinks.json" {
            w.WriteHeader(200)
            fmt.Fprintln(w, `[{"relation":["delegate_permission/common.handle_all_urls"],"target":{"namespace":"android_app","package_name":"com.myapp"}}]`)
            return
        }
        if strings.Contains(r.URL.Path, "apple-app-site-association") {
            w.WriteHeader(200)
            fmt.Fprintln(w, `{"applinks":{"details":[{"appIDs":["com.myapp"]}]}}`)
            return
        }
        w.WriteHeader(404)
    }))
    defer srv.Close()

    dl := &schema.DeepLinkCheck{URL: srv.URL + "/product/123", AndroidPackage: "com.myapp", Tier: "config-only"}
    results := CheckDeepLink(dl, "")
    if len(results) == 0 { t.Fatal("expected at least 1 assertion") }
    // Should have assetlinks + AASA checks
    passed := true
    for _, r := range results { if !r.Passed { passed = false } }
    if !passed { t.Error("expected all tier 1 checks to pass") }
}

func TestCheckDeepLink_CustomSchemeSkipsHTTP(t *testing.T) {
    dl := &schema.DeepLinkCheck{URL: "myapp://test", Tier: "config-only"}
    results := CheckDeepLink(dl, "")
    // Custom scheme should NOT trigger assetlinks/AASA HTTP checks
    for _, r := range results {
        if r.Type == "deep_link.assetlinks" || r.Type == "deep_link.aasa" {
            t.Error("custom scheme should not trigger HTTP checks")
        }
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runner/ -run "TestCheckDeepLink" -v`
Expected: FAIL — `CheckDeepLink` doesn't exist

- [ ] **Step 3: Implement CheckDeepLink**

`CheckDeepLink(cfg *schema.DeepLinkCheck, configDir string) []AssertionResult` is the main entry point:

1. Parse URL to determine scheme (custom vs https)
2. For https URLs with `check_assetlinks != false`: call `CheckAssetlinks`
3. For https URLs with `check_aasa != false`: call `CheckAASA`
4. For custom scheme URLs: check local config files (AndroidManifest, Info.plist)
5. If `tier == "full-resolve"` or (`tier == "auto"` and tools available): call `resolveDeepLink` for tier 2
6. If `tier == "full-resolve"` and no tools: return failure result

```go
func CheckDeepLink(cfg *schema.DeepLinkCheck, configDir string) []AssertionResult {
    var results []AssertionResult
    tier := cfg.Tier
    if tier == "" { tier = "auto" }

    u, _ := url.Parse(cfg.URL)
    isHTTPS := u.Scheme == "https"

    // Tier 1: HTTP checks for HTTPS URLs
    if isHTTPS {
        checkAL := cfg.CheckAssetlinks == nil || *cfg.CheckAssetlinks
        if checkAL {
            results = append(results, CheckAssetlinks(fmt.Sprintf("%s://%s", u.Scheme, u.Host), cfg.AndroidPackage))
        }
        checkAASA := cfg.CheckAASA == nil || *cfg.CheckAASA
        if checkAASA {
            results = append(results, CheckAASA(fmt.Sprintf("%s://%s", u.Scheme, u.Host), cfg.IOSBundleID))
        }
    }

    // Tier 1: Local config checks for custom schemes
    if !isHTTPS && configDir != "" {
        if r := checkLocalSchemeConfig(u.Scheme, configDir); r != nil {
            results = append(results, *r)
        }
    }

    // Tier 2: Tool-augmented resolution
    if tier == "full-resolve" || tier == "auto" {
        if hasAdb() || hasXcrun() {
            results = append(results, resolveDeepLink(cfg)...)
        } else if tier == "full-resolve" {
            results = append(results, AssertionResult{
                Type: "deep_link.resolve", Passed: false,
                Expected: "adb or xcrun available",
                Actual: "no mobile resolution tools found",
            })
        }
    }

    return results
}
```

Add helpers: `checkLocalSchemeConfig`, `hasAdb`, `hasXcrun`, `resolveDeepLink`.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/runner/ -run "TestCheckDeepLink" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/runner/assertion_deeplink.go internal/runner/assertion_deeplink_test.go
git commit -m "feat(runner): add CheckDeepLink main entry with tier routing and local config checks"
```

---

## Chunk 4: Runner Integration + Templates

### Task 5: Wire deep_link into runner.go

**Files:**
- Modify: `internal/runner/runner.go`

- [ ] **Step 1: Add deep_link assertion check to runTestOnce**

After the existing `graphql` check block in `runTestOnce`, add:

```go
if t.Expect.DeepLink != nil {
    dlResults := CheckDeepLink(t.Expect.DeepLink, r.ConfigDir)
    for _, a := range dlResults {
        assertions = append(assertions, a)
        if !a.Passed {
            allPassed = false
        }
    }
}
```

- [ ] **Step 2: Write integration test**

```go
func TestDeepLink_AssertionInRunner(t *testing.T) {
    dir := t.TempDir()
    cfg := &schema.SmokeConfig{
        Version: 1, Project: "deeplink-test",
        Tests: []schema.Test{{
            Name: "custom scheme config check",
            Expect: schema.Expect{DeepLink: &schema.DeepLinkCheck{
                URL:  "myapp://test",
                Tier: "config-only",
            }},
        }},
    }
    r := &runner.Runner{Config: cfg, Reporter: silentReporter(), ConfigDir: dir}
    result, err := r.Run(runner.RunOptions{})
    if err != nil { t.Fatal(err) }
    if result.Total != 1 { t.Errorf("total = %d", result.Total) }
}
```

- [ ] **Step 3: Run test**

Run: `go test ./internal/runner/ -run TestDeepLink_AssertionInRunner -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/runner/runner.go internal/runner/assertion_deeplink_test.go
git commit -m "feat(runner): wire deep_link assertion into test execution pipeline"
```

---

### Task 6: Add mobile templates to detector

**Files:**
- Modify: `internal/detector/templates.go`

- [ ] **Step 1: Add mobile project templates to GenerateConfig switch**

Add cases for `ReactNative`, `Flutter`, `IOS`, `Android` in the switch statement:

```go
case ReactNative:
    cfg.Tests = append(cfg.Tests, schema.Test{
        Name: "Deep link scheme configured",
        Expect: schema.Expect{DeepLink: &schema.DeepLinkCheck{
            URL:  filepath.Base(dir) + "://test",
            Tier: "config-only",
        }},
    })

case Flutter, IOS, Android:
    cfg.Tests = append(cfg.Tests, schema.Test{
        Name: "Universal link config valid",
        Expect: schema.Expect{DeepLink: &schema.DeepLinkCheck{
            URL:              "https://" + filepath.Base(dir) + ".com",
            CheckAssetlinks: boolPtr(true),
            CheckAASA:       boolPtr(true),
            Tier:            "auto",
        }},
    })
```

- [ ] **Step 2: Run detector tests**

Run: `go test ./internal/detector/ -v`
Expected: All pass

- [ ] **Step 3: Commit**

```bash
git add internal/detector/templates.go
git commit -m "feat(detector): add mobile project smoke init templates with deep link checks"
```

---

### Task 7: Update CLAUDE.md and validate

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add `deep_link` to assertion types table**

Add row:

```
| deep_link | `{url, android_package?, ios_bundle_id?, ios_associated_domains?, check_assetlinks?, check_aasa?, tier?}` | Mobile deep link / universal link verification (two-tier: HTTP config + tool-augmented resolution) |
```

- [ ] **Step 2: Add mobile project types to detected types**

Add to "Detected Project Types" section: React Native, Flutter, iOS, Android

- [ ] **Step 3: Run full test suite**

Run: `go test ./...`
Expected: All pass

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add deep_link assertion and mobile detector types to CLAUDE.md"
```
