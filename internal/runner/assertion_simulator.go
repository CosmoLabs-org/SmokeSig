package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// simctlDeviceList represents the JSON output of `xcrun simctl list devices -j`.
type simctlDeviceList struct {
	Devices map[string][]simctlDevice `json:"devices"`
}

// simctlDevice represents a single iOS simulator device.
type simctlDevice struct {
	Name  string `json:"name"`
	UDID  string `json:"udid"`
	State string `json:"state"`
}

// CheckIOSSimulator verifies that a booted iOS simulator is available.
// Runs `xcrun simctl list devices -j`, parses the JSON output, and looks for
// a device with state "Booted". Optionally filters by device name and OS version.
func CheckIOSSimulator(check *schema.IOSSimulatorCheck) AssertionResult {
	timeout := check.Timeout.Duration
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "xcrun", "simctl", "list", "devices", "-j")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return AssertionResult{
			Type:     "ios_simulator",
			Expected: "booted iOS simulator",
			Actual:   fmt.Sprintf("xcrun simctl failed: %s", msg),
			Passed:   false,
		}
	}

	booted, actual := parseSimctlOutput(out, check.DeviceName, check.OS)
	if !booted {
		return AssertionResult{
			Type:     "ios_simulator",
			Expected: formatIOSExpected(check),
			Actual:   actual,
			Passed:   false,
		}
	}

	return AssertionResult{
		Type:     "ios_simulator",
		Expected: formatIOSExpected(check),
		Actual:   actual,
		Passed:   true,
	}
}

// parseSimctlOutput parses the xcrun simctl JSON and looks for a booted device
// matching the optional filters. Returns (found, description).
func parseSimctlOutput(data []byte, deviceName, osFilter string) (bool, string) {
	var deviceList simctlDeviceList
	if err := json.Unmarshal(data, &deviceList); err != nil {
		return false, fmt.Sprintf("failed to parse simctl JSON: %s", err)
	}

	var bootedDevices []string

	for runtime, devices := range deviceList.Devices {
		for _, d := range devices {
			if d.State != "Booted" {
				continue
			}

			// Apply OS filter: runtime keys look like "com.apple.CoreSimulator.SimRuntime.iOS-17-4"
			if osFilter != "" {
				normalizedRuntime := strings.ReplaceAll(runtime, "-", ".")
				normalizedFilter := strings.ReplaceAll(osFilter, "-", ".")
				if !strings.Contains(normalizedRuntime, normalizedFilter) {
					continue
				}
			}

			// Apply device name filter
			if deviceName != "" && !strings.Contains(d.Name, deviceName) {
				continue
			}

			bootedDevices = append(bootedDevices, fmt.Sprintf("%s (%s)", d.Name, d.UDID))
		}
	}

	if len(bootedDevices) == 0 {
		return false, "no booted simulator found"
	}

	return true, fmt.Sprintf("booted: %s", strings.Join(bootedDevices, ", "))
}

// formatIOSExpected builds the Expected string for iOS simulator checks.
func formatIOSExpected(check *schema.IOSSimulatorCheck) string {
	parts := []string{"booted iOS simulator"}
	if check.DeviceName != "" {
		parts = append(parts, fmt.Sprintf("name=%q", check.DeviceName))
	}
	if check.OS != "" {
		parts = append(parts, fmt.Sprintf("os=%q", check.OS))
	}
	return strings.Join(parts, ", ")
}

// CheckAndroidEmulator verifies that an Android emulator has finished booting.
// Runs `adb shell getprop sys.boot_completed` and checks that the output is "1".
// If Serial is provided, targets a specific device with `adb -s <serial>`.
func CheckAndroidEmulator(check *schema.AndroidEmulatorCheck) AssertionResult {
	timeout := check.Timeout.Duration
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := []string{}
	if check.Serial != "" {
		args = append(args, "-s", check.Serial)
	}
	args = append(args, "shell", "getprop", "sys.boot_completed")

	cmd := exec.CommandContext(ctx, "adb", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return AssertionResult{
			Type:     "android_emulator",
			Expected: formatAndroidExpected(check),
			Actual:   fmt.Sprintf("adb failed: %s", msg),
			Passed:   false,
		}
	}

	value := strings.TrimSpace(string(out))
	if value != "1" {
		return AssertionResult{
			Type:     "android_emulator",
			Expected: formatAndroidExpected(check),
			Actual:   fmt.Sprintf("sys.boot_completed=%q (not ready)", value),
			Passed:   false,
		}
	}

	actual := "boot completed"
	if check.Serial != "" {
		actual = fmt.Sprintf("boot completed (serial=%s)", check.Serial)
	}
	return AssertionResult{
		Type:     "android_emulator",
		Expected: formatAndroidExpected(check),
		Actual:   actual,
		Passed:   true,
	}
}

// formatAndroidExpected builds the Expected string for Android emulator checks.
func formatAndroidExpected(check *schema.AndroidEmulatorCheck) string {
	if check.Serial != "" {
		return fmt.Sprintf("android emulator ready (serial=%s)", check.Serial)
	}
	return "android emulator ready"
}
