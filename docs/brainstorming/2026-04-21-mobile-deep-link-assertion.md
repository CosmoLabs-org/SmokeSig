---
name: Mobile Deep Link Assertion
description: Two-tier progressive deep link assertions for Android, iOS, and cross-platform mobile projects
type: project
created: 2026-04-21
origin_idea: IDEA-033
---

# Mobile Deep Link Assertion

## Goal

Add deep link verification to cosmo-smoke with a progressive two-tier model: zero-dep config checks that always work, and tool-augmented resolution tests when emulators/devices are available.

## Problem

Mobile teams using React Native, Flutter, or native iOS/Android need to verify deep links and universal links resolve correctly. Current smoke test tooling has no mobile assertion types. cosmo-smoke's zero-deps philosophy conflicts with emulator requirements — this design resolves the tension with a progressive depth model.

## Design Decisions

### Decision 1: Two-tier progressive model

**Tier 1 — `deep_link` (zero-deps, always works):**
- Validates `assetlinks.json` (Android App Links) and `apple-app-site-association` (iOS Universal Links) via HTTPS HEAD/GET
- Checks URL scheme configuration in local project files:
  - Android: `AndroidManifest.xml` intent-filter data elements
  - iOS: `Info.plist` CFBundleURLSchemes
  - React Native: `app.json` deep link configuration
  - Flutter: `android/app/src/main/AndroidManifest.xml` + `ios/Runner/Info.plist`
- Detects mobile project type via `internal/detector` markers
- No emulator, device, or external tools required

**Tier 2 — tool-augmented resolution (opt-in):**
- When `adb` available: `adb shell am start -a android.intent.action.VIEW -d <url>` — checks resolution against expected package
- When `xcrun` available: `xcrun simctl openurl booted <url>` — verifies simulator handles the URL
- Falls back gracefully with skip message when tools absent
- Controlled by `tier` field: `auto` (default), `config-only`, `full-resolve`

**Why two tiers:** Tier 1 catches 80% of deep link bugs (misconfigured domains, missing files, wrong schemes) without any setup. Tier 2 catches the remaining 20% (runtime resolution failures, app not installed, wrong activity) when the team has CI emulators.

### Decision 2: Single `deep_link` assertion type

One assertion type with auto-tier detection rather than separate `android_deep_link` / `ios_deep_link`. Reasons:
- Same `url` parameter works across platforms
- Auto-detection from project files determines which checks to run
- Users writing cross-platform tests don't duplicate configuration
- Simpler YAML, easier to understand

### Decision 3: No new external dependencies

All checks use:
- `net/http` for fetching `assetlinks.json` / `apple-app-site-association`
- `encoding/json` for parsing JSON configs
- `encoding/xml` for AndroidManifest.xml
- `os/exec` for `adb` / `xcrun` (already used by runner for test execution)
- `os` for file existence checks

## Assertion Schema

```yaml
tests:
  - name: "Product deep link resolves"
    deep_link:
      url: "myapp://product/123"
      android_package: "com.myapp"           # optional, for tier 2
      ios_bundle_id: "com.myapp"             # optional, for tier 2
      ios_associated_domains:                # optional, for tier 1 iOS checks
        - "applinks:myapp.com"
      tier: auto                             # auto | config-only | full-resolve

  - name: "Universal link config valid"
    deep_link:
      url: "https://myapp.com/product/123"
      check_assetlinks: true                 # verify .well-known/assetlinks.json
      check_aasa: true                       # verify apple-app-site-association
```

### Field Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `url` | string | yes | Deep link or universal link URL to verify |
| `android_package` | string | no | Expected Android package name (tier 2) |
| `ios_bundle_id` | string | no | Expected iOS bundle identifier (tier 2) |
| `ios_associated_domains` | []string | no | Expected applinks domains for AASA validation |
| `check_assetlinks` | bool | no | Verify `assetlinks.json` at the URL's host (default: true for https URLs) |
| `check_aasa` | bool | no | Verify `apple-app-site-association` at the URL's host (default: true for https URLs) |
| `tier` | string | no | `auto` (default), `config-only`, `full-resolve` |

### Tier Behavior

| Tier | `adb` available | `xcrun` available | What runs |
|------|----------------|-------------------|-----------|
| `auto` | no | no | Tier 1 only (config + HTTP checks) |
| `auto` | yes | no | Tier 1 + Android resolution |
| `auto` | no | yes | Tier 1 + iOS resolution |
| `auto` | yes | yes | Tier 1 + both platform resolutions |
| `config-only` | — | — | Tier 1 only, always |
| `full-resolve` | no | no | **Fail** with "no mobile tools available" |
| `full-resolve` | yes/no | yes/no | Tier 1 + whatever platform tools found |

## Mobile Project Detection

Extends `internal/detector` with 4 new types:

| Type | Markers |
|------|---------|
| `ReactNative` | `app.json` + (`metro.config.js` or `react-native` in `package.json` dependencies) |
| `Flutter` | `pubspec.yaml` with `flutter` in dependencies |
| `iOS` | `*.xcodeproj` or `*.xcworkspace` or `Podfile` |
| `Android` | `build.gradle` or `build.gradle.kts` (without `go.mod` or `package.json` nearby) |

Detection priority: `ReactNative` and `Flutter` take precedence over raw `iOS`/`Android` (a Flutter project has `build.gradle` but should be detected as Flutter).

## Tier 1 Checks (Detail)

### Android App Links — `assetlinks.json`

For URLs with `https://` scheme:
1. GET `https://{host}/.well-known/assetlinks.json`
2. Parse JSON array of statements
3. Verify at least one statement has:
   - `relation: ["delegate_permission/common.handle_all_urls"]`
   - `target.namespace: "android_app"`
   - `target.package_name` matches `android_package` (if specified)

### iOS Universal Links — `apple-app-site-association`

For URLs with `https://` scheme:
1. GET `https://{host}/apple-app-site-association` (or `/.well-known/apple-app-site-association`)
2. Parse JSON
3. Verify `applinks.details` contains an entry matching `ios_bundle_id` (if specified)
4. Verify the path pattern covers the URL's path

### URL Scheme Config — Local Files

For custom scheme URLs (`myapp://...`):
1. Detect project type from `internal/detector`
2. Check the relevant config file for the scheme:
   - Android: parse `AndroidManifest.xml` for `<data android:scheme="myapp"/>`
   - iOS: parse `Info.plist` for `CFBundleURLSchemes` containing `"myapp"`
   - React Native: check `app.json` linking configuration
   - Flutter: check both Android and iOS manifests

## Tier 2 Checks (Detail)

### Android — `adb`

```bash
adb shell am start -a android.intent.action.VIEW -d "myapp://product/123"
```

Parse output for:
- Success: `Starting: Intent { ... }` with no error
- Resolution failure: `Error: Activity does not exist` or `android.content.ActivityNotFoundException`
- Multiple handlers: warn (ambiguous resolution)

Expected result: resolved to `android_package` if specified.

### iOS — `xcrun simctl`

```bash
xcrun simctl openurl booted "myapp://product/123"
```

Parse output:
- Success: no error output
- Failure: `Unable to find application` or non-zero exit code

Note: iOS doesn't tell you which app opened the URL — the test verifies the URL was accepted by the simulator, not that a specific app received it. The `ios_bundle_id` check is best-effort.

## `smoke init` Templates

When a mobile project type is detected, `smoke init` generates:

**React Native:**
```yaml
tests:
  - name: "Deep link scheme configured"
    deep_link:
      url: "myapp://test"
      tier: config-only
```

**Flutter / iOS / Android (with HTTPS):**
```yaml
tests:
  - name: "Universal link config valid"
    deep_link:
      url: "https://myapp.com"
      check_assetlinks: true
      check_aasa: true
      tier: auto
```

## File Structure

```
internal/
  runner/
    assertion_deeplink.go           # NEW: Tier 1 + Tier 2 assertion logic
    assertion_deeplink_test.go      # NEW: Tests with mock HTTP server
  detector/
    detector.go                     # MODIFY: Add ReactNative, Flutter, iOS, Android types
    templates.go                    # MODIFY: Mobile project templates
  schema/
    schema.go                       # MODIFY: Add DeepLink struct to Expect
```

## Estimated Scope

| Component | Files | LOC (new) | Time |
|-----------|-------|-----------|------|
| DeepLink assertion (tier 1 + 2) | 2 | ~400 | 2h |
| Tests (unit + integration) | 1 | ~300 | 1.5h |
| Detector types + templates | 2 | ~120 | 45m |
| Schema struct | 1 | ~30 | 15m |
| **Total** | **6** | **~850** | **~4.5h** |

## Out of Scope (v1)

- Deferred deep link testing (post-install attribution)
- Branch.io / Firebase Dynamic Links provider-specific checks
- Push notification deep link verification
- App clip / instant app deep links
- QR code deep link generation
