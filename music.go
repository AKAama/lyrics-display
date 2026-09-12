package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func readNowPlaying(ctx context.Context) (nowPlaying, error) {
	if !musicAppRunning(ctx) {
		return nowPlaying{State: stateStopped}, nil
	}

	script := `
on sanitizeText(value)
	set textValue to value as text
	set textValue to my replaceText(return, " ", textValue)
	set textValue to my replaceText(linefeed, " ", textValue)
	set textValue to my replaceText(character id 31, " ", textValue)
	return textValue
end sanitizeText

on replaceText(findText, replaceText, subject)
	set AppleScript's text item delimiters to findText
	set textItems to every text item of subject
	set AppleScript's text item delimiters to replaceText
	set subject to textItems as text
	set AppleScript's text item delimiters to ""
	return subject
end replaceText

tell application "Music"
	set currentState to player state as text
	if currentState is "stopped" then
		return "stopped" & character id 31 & "" & character id 31 & "" & character id 31 & "" & character id 31 & "0"
	end if

	set trackName to my sanitizeText(name of current track)
	set artistName to my sanitizeText(artist of current track)
	set albumName to my sanitizeText(album of current track)
	set playerPosition to player position
	set trackDuration to 0
	try
		set trackDuration to duration of current track
	end try

	return currentState & character id 31 & trackName & character id 31 & artistName & character id 31 & albumName & character id 31 & (playerPosition as text) & character id 31 & (trackDuration as text)
end tell
`

	cmd := exec.CommandContext(ctx, "osascript", "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nowPlaying{}, fmt.Errorf("osascript failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nowPlaying{}, err
	}

	parts := strings.Split(strings.TrimSpace(string(out)), fieldSeparator)
	if len(parts) < 5 {
		return nowPlaying{}, fmt.Errorf("unexpected player payload: %q", string(out))
	}

	seconds, err := parseAppleScriptNumber(parts[4])
	if err != nil {
		return nowPlaying{}, fmt.Errorf("parse player position: %w", err)
	}

	var duration time.Duration
	if len(parts) >= 6 {
		if total, durErr := parseAppleScriptNumber(parts[5]); durErr == nil {
			duration = time.Duration(total * float64(time.Second))
		}
	}

	return nowPlaying{
		State:    playerState(strings.TrimSpace(parts[0])),
		Track:    strings.TrimSpace(parts[1]),
		Artist:   strings.TrimSpace(parts[2]),
		Album:    strings.TrimSpace(parts[3]),
		Position: time.Duration(seconds * float64(time.Second)),
		Duration: duration,
	}, nil
}

func parseAppleScriptNumber(value string) (float64, error) {
	value = strings.TrimSpace(value)
	if strings.Contains(value, ",") && !strings.Contains(value, ".") {
		value = strings.ReplaceAll(value, ",", ".")
	}
	return strconv.ParseFloat(value, 64)
}

func musicAppRunning(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "pgrep", "-x", "Music")
	return cmd.Run() == nil
}

var errAutomationDenied = errors.New("automation permission denied")

func readBuiltinLyrics(ctx context.Context) (string, error) {
	if !musicAppRunning(ctx) {
		return "", nil
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	script := `
tell application "Music"
	try
		if player state is stopped then return ""
		set l to lyrics of current track
		if l is missing value then return ""
		return l as text
	on error errMsg number errNum
		return "__ERROR__" & errNum & ":" & errMsg
	end try
end tell
`

	cmd := exec.CommandContext(ctx, "osascript", "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			msg := strings.TrimSpace(string(exitErr.Stderr))
			if isAutomationDenied(msg) {
				return "", errAutomationDenied
			}
			return "", fmt.Errorf("osascript failed: %s", msg)
		}
		return "", err
	}

	text := strings.TrimSpace(string(out))
	if strings.HasPrefix(text, "__ERROR__") {
		msg := strings.TrimPrefix(text, "__ERROR__")
		if isAutomationDenied(msg) {
			return "", errAutomationDenied
		}
		return "", fmt.Errorf("read builtin lyrics: %s", msg)
	}

	return text, nil
}

func isAutomationDenied(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "not allowed") ||
		strings.Contains(lower, "not authorised") ||
		strings.Contains(lower, "not authorized") ||
		strings.Contains(msg, "1743") ||
		strings.Contains(msg, "-1743")
}
