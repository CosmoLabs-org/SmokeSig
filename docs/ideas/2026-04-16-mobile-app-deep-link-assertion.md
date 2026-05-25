---
id: IDEA-033
legacy_id: IDEA-MO1FC22M
title: Mobile app deep link assertion
created: "2026-04-16T08:56:49.150253-03:00"
status: harvested
source: human
---


# Mobile app deep link assertion

Verify deep links resolve correctly for iOS/Android apps. Essential for mobile teams using React Native, Flutter.

## Scope

This is part of a broader effort to support multiple project types beyond the current set (Go, Node, Python, Docker, Rust). Mobile project types (React Native, Flutter, native iOS/Android) would be auto-detected by the `internal/detector` package via markers like `app.json` + `metro.config.js` (RN), `pubspec.yaml` (Flutter), `*.xcodeproj` (iOS), `build.gradle` (Android).

## Implementation path

1. Add mobile project types to `internal/detector`
2. `smoke init` generates deep link assertions based on host tooling:
   - `adb` available → Android intent resolution tests
   - `xcrun` available → iOS universal link tests
   - Neither → fallback to HTTP checks on `assetlinks.json` / `apple-app-site-association`
3. Assertions are environment-aware — same `.smoke.yaml` works across CI (emulators) and local dev (devices)
