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

	return currentState & character id 31 & trackName & character id 31 & artistName & character id 31 & albumName & character id 31 & (playerPosition as text)
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
	if len(parts) != 5 {
		return nowPlaying{}, fmt.Errorf("unexpected player payload: %q", string(out))
	}

	seconds, err := strconv.ParseFloat(strings.TrimSpace(parts[4]), 64)
	if err != nil {
		return nowPlaying{}, fmt.Errorf("parse player position: %w", err)
	}

	return nowPlaying{
		State:    playerState(strings.TrimSpace(parts[0])),
		Track:    strings.TrimSpace(parts[1]),
		Artist:   strings.TrimSpace(parts[2]),
		Album:    strings.TrimSpace(parts[3]),
		Position: time.Duration(seconds * float64(time.Second)),
	}, nil
}

func musicAppRunning(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "pgrep", "-x", "Music")
	return cmd.Run() == nil
}
