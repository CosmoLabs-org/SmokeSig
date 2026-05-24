package runner

import (
	"strings"
	"testing"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// --- CheckIOSSimulator ---

func TestCheckIOSSimulator_TypeField(t *testing.T) {
	result := CheckIOSSimulator(&schema.IOSSimulatorCheck{})
	if result.Type != "ios_simulator" {
		t.Errorf("type = %q, want ios_simulator", result.Type)
	}
	// xcrun may not be available in test env — just verify no panic and correct type
}

func TestCheckIOSSimulator_DefaultTimeout(t *testing.T) {
	check := &schema.IOSSimulatorCheck{}
	// Verify the default timeout is applied internally (10s)
	// We can't directly inspect it, but we verify the function runs without panic
	result := CheckIOSSimulator(check)
	if result.Type != "ios_simulator" {
		t.Errorf("type = %q, want ios_simulator", result.Type)
	}
}

func TestCheckIOSSimulator_CustomTimeout(t *testing.T) {
	result := CheckIOSSimulator(&schema.IOSSimulatorCheck{
		Timeout: schema.Duration{Duration: 2 * time.Second},
	})
	if result.Type != "ios_simulator" {
		t.Errorf("type = %q, want ios_simulator", result.Type)
	}
}

func TestCheckIOSSimulator_WithDeviceFilter(t *testing.T) {
	result := CheckIOSSimulator(&schema.IOSSimulatorCheck{
		DeviceName: "iPhone 15",
	})
	if result.Type != "ios_simulator" {
		t.Errorf("type = %q, want ios_simulator", result.Type)
	}
	if result.Expected == "" {
		t.Error("expected non-empty Expected field")
	}
}

func TestCheckIOSSimulator_WithOSFilter(t *testing.T) {
	result := CheckIOSSimulator(&schema.IOSSimulatorCheck{
		OS: "iOS-17",
	})
	if result.Type != "ios_simulator" {
		t.Errorf("type = %q, want ios_simulator", result.Type)
	}
}

// --- parseSimctlOutput ---

func TestParseSimctlOutput_BootedDevice(t *testing.T) {
	jsonData := []byte(`{
		"devices": {
			"com.apple.CoreSimulator.SimRuntime.iOS-17-4": [
				{"name": "iPhone 15", "udid": "ABC-123", "state": "Booted"},
				{"name": "iPhone 15 Pro", "udid": "DEF-456", "state": "Shutdown"}
			],
			"com.apple.CoreSimulator.SimRuntime.iOS-16-4": [
				{"name": "iPhone 14", "udid": "GHI-789", "state": "Shutdown"}
			]
		}
	}`)

	found, actual := parseSimctlOutput(jsonData, "", "")
	if !found {
		t.Errorf("expected to find booted device, got: %s", actual)
	}
	if actual == "" {
		t.Error("expected non-empty actual description")
	}
}

func TestParseSimctlOutput_NoBootedDevice(t *testing.T) {
	jsonData := []byte(`{
		"devices": {
			"com.apple.CoreSimulator.SimRuntime.iOS-17-4": [
				{"name": "iPhone 15", "udid": "ABC-123", "state": "Shutdown"},
				{"name": "iPhone 15 Pro", "udid": "DEF-456", "state": "Shutdown"}
			]
		}
	}`)

	found, actual := parseSimctlOutput(jsonData, "", "")
	if found {
		t.Errorf("expected no booted device, got: %s", actual)
	}
	if actual != "no booted simulator found" {
		t.Errorf("actual = %q, want 'no booted simulator found'", actual)
	}
}

func TestParseSimctlOutput_FilterByDeviceName(t *testing.T) {
	jsonData := []byte(`{
		"devices": {
			"com.apple.CoreSimulator.SimRuntime.iOS-17-4": [
				{"name": "iPhone 15", "udid": "ABC-123", "state": "Booted"},
				{"name": "iPhone 15 Pro", "udid": "DEF-456", "state": "Booted"}
			]
		}
	}`)

	// Filter for "iPhone 15 Pro" only
	found, actual := parseSimctlOutput(jsonData, "iPhone 15 Pro", "")
	if !found {
		t.Errorf("expected to find booted 'iPhone 15 Pro', got: %s", actual)
	}
	if found && !strings.Contains(actual, "DEF-456") {
		t.Errorf("expected actual to contain DEF-456, got: %s", actual)
	}
}

func TestParseSimctlOutput_FilterByDeviceName_NoMatch(t *testing.T) {
	jsonData := []byte(`{
		"devices": {
			"com.apple.CoreSimulator.SimRuntime.iOS-17-4": [
				{"name": "iPhone 15", "udid": "ABC-123", "state": "Booted"}
			]
		}
	}`)

	found, _ := parseSimctlOutput(jsonData, "iPad Pro", "")
	if found {
		t.Error("expected no match for 'iPad Pro' filter")
	}
}

func TestParseSimctlOutput_FilterByOS(t *testing.T) {
	jsonData := []byte(`{
		"devices": {
			"com.apple.CoreSimulator.SimRuntime.iOS-17-4": [
				{"name": "iPhone 15", "udid": "ABC-123", "state": "Booted"}
			],
			"com.apple.CoreSimulator.SimRuntime.iOS-16-4": [
				{"name": "iPhone 14", "udid": "GHI-789", "state": "Booted"}
			]
		}
	}`)

	// Filter for iOS 17 only
	found, actual := parseSimctlOutput(jsonData, "", "iOS-17")
	if !found {
		t.Errorf("expected to find booted iOS 17 device, got: %s", actual)
	}
	if found && !strings.Contains(actual, "ABC-123") {
		t.Errorf("expected actual to contain ABC-123, got: %s", actual)
	}
}

func TestParseSimctlOutput_FilterByOS_NoMatch(t *testing.T) {
	jsonData := []byte(`{
		"devices": {
			"com.apple.CoreSimulator.SimRuntime.iOS-17-4": [
				{"name": "iPhone 15", "udid": "ABC-123", "state": "Booted"}
			]
		}
	}`)

	found, _ := parseSimctlOutput(jsonData, "", "iOS-16")
	if found {
		t.Error("expected no match for iOS-16 filter")
	}
}

func TestParseSimctlOutput_CombinedFilters(t *testing.T) {
	jsonData := []byte(`{
		"devices": {
			"com.apple.CoreSimulator.SimRuntime.iOS-17-4": [
				{"name": "iPhone 15", "udid": "ABC-123", "state": "Booted"},
				{"name": "iPad Pro", "udid": "XYZ-999", "state": "Booted"}
			],
			"com.apple.CoreSimulator.SimRuntime.iOS-16-4": [
				{"name": "iPhone 15", "udid": "GHI-789", "state": "Booted"}
			]
		}
	}`)

	// Filter for iPhone 15 on iOS 17
	found, actual := parseSimctlOutput(jsonData, "iPhone 15", "iOS-17")
	if !found {
		t.Errorf("expected to find iPhone 15 on iOS 17, got: %s", actual)
	}
	if found && !strings.Contains(actual, "ABC-123") {
		t.Errorf("expected actual to contain ABC-123 (iPhone 15 on iOS 17), got: %s", actual)
	}
	// Should NOT contain GHI-789 (iPhone 15 on iOS 16)
	if found && strings.Contains(actual, "GHI-789") {
		t.Errorf("should not contain GHI-789 (wrong OS), got: %s", actual)
	}
}

func TestParseSimctlOutput_InvalidJSON(t *testing.T) {
	found, actual := parseSimctlOutput([]byte("not json"), "", "")
	if found {
		t.Error("expected failure on invalid JSON")
	}
	if !strings.Contains(actual, "failed to parse") {
		t.Errorf("expected parse error message, got: %s", actual)
	}
}

func TestParseSimctlOutput_EmptyDevices(t *testing.T) {
	jsonData := []byte(`{"devices": {}}`)
	found, actual := parseSimctlOutput(jsonData, "", "")
	if found {
		t.Error("expected no booted device in empty list")
	}
	if actual != "no booted simulator found" {
		t.Errorf("actual = %q, want 'no booted simulator found'", actual)
	}
}

// --- formatIOSExpected ---

func TestFormatIOSExpected_NoFilters(t *testing.T) {
	result := formatIOSExpected(&schema.IOSSimulatorCheck{})
	if result != "booted iOS simulator" {
		t.Errorf("got %q, want 'booted iOS simulator'", result)
	}
}

func TestFormatIOSExpected_WithDeviceName(t *testing.T) {
	result := formatIOSExpected(&schema.IOSSimulatorCheck{DeviceName: "iPhone 15"})
	if !strings.Contains(result, "iPhone 15") {
		t.Errorf("expected result to contain device name, got: %s", result)
	}
}

func TestFormatIOSExpected_WithOS(t *testing.T) {
	result := formatIOSExpected(&schema.IOSSimulatorCheck{OS: "iOS-17"})
	if !strings.Contains(result, "iOS-17") {
		t.Errorf("expected result to contain OS, got: %s", result)
	}
}

// --- CheckAndroidEmulator ---

func TestCheckAndroidEmulator_TypeField(t *testing.T) {
	result := CheckAndroidEmulator(&schema.AndroidEmulatorCheck{})
	if result.Type != "android_emulator" {
		t.Errorf("type = %q, want android_emulator", result.Type)
	}
	// adb may not be available in test env — just verify no panic and correct type
}

func TestCheckAndroidEmulator_DefaultTimeout(t *testing.T) {
	check := &schema.AndroidEmulatorCheck{}
	result := CheckAndroidEmulator(check)
	if result.Type != "android_emulator" {
		t.Errorf("type = %q, want android_emulator", result.Type)
	}
}

func TestCheckAndroidEmulator_CustomTimeout(t *testing.T) {
	result := CheckAndroidEmulator(&schema.AndroidEmulatorCheck{
		Timeout: schema.Duration{Duration: 2 * time.Second},
	})
	if result.Type != "android_emulator" {
		t.Errorf("type = %q, want android_emulator", result.Type)
	}
}

func TestCheckAndroidEmulator_WithSerial(t *testing.T) {
	result := CheckAndroidEmulator(&schema.AndroidEmulatorCheck{
		Serial: "emulator-5554",
	})
	if result.Type != "android_emulator" {
		t.Errorf("type = %q, want android_emulator", result.Type)
	}
	if result.Expected == "" {
		t.Error("expected non-empty Expected field")
	}
	if !strings.Contains(result.Expected, "emulator-5554") {
		t.Errorf("expected Expected to contain serial, got: %s", result.Expected)
	}
}

// --- formatAndroidExpected ---

func TestFormatAndroidExpected_NoSerial(t *testing.T) {
	result := formatAndroidExpected(&schema.AndroidEmulatorCheck{})
	if result != "android emulator ready" {
		t.Errorf("got %q, want 'android emulator ready'", result)
	}
}

func TestFormatAndroidExpected_WithSerial(t *testing.T) {
	result := formatAndroidExpected(&schema.AndroidEmulatorCheck{Serial: "emulator-5554"})
	if !strings.Contains(result, "emulator-5554") {
		t.Errorf("expected result to contain serial, got: %s", result)
	}
}


