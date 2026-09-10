package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const brewServiceLabel = "homebrew.mxcl.lyrics-display"

var launchdPIDPattern = regexp.MustCompile(`(?m)^\s*pid\s*=\s*(\d+)`)

func detectServiceManaged(flagSet bool) bool {
	if flagSet {
		return true
	}
	if isBrewXPCService(os.Getenv("XPC_SERVICE_NAME")) {
		return true
	}
	return launchdJobOwnsPID(brewServiceLabel, os.Getpid())
}

func isBrewXPCService(name string) bool {
	return strings.Contains(name, brewServiceLabel)
}

func launchdJobOwnsPID(label string, pid int) bool {
	uid := os.Getuid()
	out, err := exec.Command("launchctl", "print", fmt.Sprintf("gui/%d/%s", uid, label)).Output()
	if err != nil {
		return false
	}
	jobPID, ok := parseLaunchdJobPID(string(out))
	return ok && jobPID == pid
}

func parseLaunchdJobPID(output string) (int, bool) {
	match := launchdPIDPattern.FindStringSubmatch(output)
	if len(match) < 2 {
		return 0, false
	}
	pid, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, false
	}
	return pid, true
}

func stopBrewLaunchAgent() error {
	uid := os.Getuid()
	target := fmt.Sprintf("gui/%d/%s", uid, brewServiceLabel)
	if out, err := exec.Command("launchctl", "bootout", target).CombinedOutput(); err == nil {
		return nil
	} else {
		home, homeErr := os.UserHomeDir()
		if homeErr == nil {
			plist := filepath.Join(home, "Library", "LaunchAgents", brewServiceLabel+".plist")
			if _, unloadErr := exec.Command("launchctl", "unload", plist).CombinedOutput(); unloadErr == nil {
				return nil
			}
		}
		if brewErr := stopViaBrew(); brewErr == nil {
			return nil
		}
		return fmt.Errorf("launchctl bootout %s: %s", target, strings.TrimSpace(string(out)))
	}
}

func stopViaBrew() error {
	brew := findBrew()
	if brew == "" {
		return fmt.Errorf("brew not found")
	}

	names := []string{
		"akaama/lyrics-display/lyrics-display",
		"lyrics-display",
	}
	var lastErr error
	for _, name := range names {
		cmd := exec.Command(brew, "services", "stop", name)
		cmd.Env = append(os.Environ(), "PATH="+brewPathDir(brew)+":"+os.Getenv("PATH"))
		if out, err := cmd.CombinedOutput(); err == nil {
			return nil
		} else {
			lastErr = fmt.Errorf("%s: %s", name, strings.TrimSpace(string(out)))
		}
	}
	return lastErr
}

func findBrew() string {
	if path, err := exec.LookPath("brew"); err == nil {
		return path
	}
	candidates := []string{
		"/opt/homebrew/bin/brew",
		"/usr/local/bin/brew",
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func brewPathDir(brew string) string {
	return filepath.Dir(brew)
}
