package main

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ttmlPPattern     = regexp.MustCompile(`(?s)<p\s+([^>]*)>(.*?)</p>`)
	ttmlBeginPattern = regexp.MustCompile(`\bbegin="([^"]+)"`)
	ttmlTagPattern   = regexp.MustCompile(`<[^>]+>`)
	ttmlSongID       = regexp.MustCompile(`(?:[?&]id=|/songs/)(\d+)`)
)

func parseTTML(raw string) []lyricLine {
	var lines []lyricLine
	for _, match := range ttmlPPattern.FindAllStringSubmatch(raw, -1) {
		begin := ttmlBeginPattern.FindStringSubmatch(match[1])
		if begin == nil {
			continue
		}
		text := strings.TrimSpace(ttmlTagPattern.ReplaceAllString(match[2], ""))
		text = strings.Join(strings.Fields(text), " ")
		if text == "" {
			continue
		}
		at, err := parseTTMLClock(begin[1])
		if err != nil {
			continue
		}
		lines = append(lines, lyricLine{At: at, Text: text})
	}
	sort.Slice(lines, func(i, j int) bool {
		return lines[i].At < lines[j].At
	})
	return lines
}

func parseTTMLClock(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, "s")
	if raw == "" {
		return 0, fmt.Errorf("empty ttml clock")
	}

	parts := strings.Split(raw, ":")
	var hours, minutes int
	var seconds float64
	var err error

	switch len(parts) {
	case 1:
		seconds, err = strconv.ParseFloat(parts[0], 64)
	case 2:
		minutes, err = strconv.Atoi(parts[0])
		if err == nil {
			seconds, err = strconv.ParseFloat(parts[1], 64)
		}
	case 3:
		hours, err = strconv.Atoi(parts[0])
		if err == nil {
			minutes, err = strconv.Atoi(parts[1])
		}
		if err == nil {
			seconds, err = strconv.ParseFloat(parts[2], 64)
		}
	default:
		return 0, fmt.Errorf("invalid ttml clock %q", raw)
	}
	if err != nil {
		return 0, err
	}

	total := time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds*float64(time.Second))
	return total, nil
}

func appleSongIDFromURL(raw string) string {
	match := ttmlSongID.FindStringSubmatch(raw)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}
