package main

import (
	"os/exec"
	"strings"
	"time"
	"unicode"
)

const (
	defaultMenuBarSlotCells = defaultSlotWidth * 2
	marqueeTickInterval     = 180 * time.Millisecond
	marqueeHoldDuration     = 1100 * time.Millisecond
	menuBarPadFull          = '\u3000' // ideographic space, ~1 CJK em
	menuBarPadHalf          = '\u2007' // figure space, ~1 ASCII digit
)

type marqueeState struct {
	text         string
	offset       int
	dir          int
	holdUntil    time.Time
	slotCells    int
	reduceMotion bool
}

func newMarqueeState(slotCells int) marqueeState {
	return marqueeState{
		dir:          1,
		slotCells:    normalizeSlotCells(slotCells),
		reduceMotion: systemReduceMotionEnabled(),
	}
}

func (m *marqueeState) cells() int {
	return normalizeSlotCells(m.slotCells)
}

func (m *marqueeState) setText(text string, now time.Time) {
	text = strings.TrimSpace(text)
	if m.text == text {
		return
	}
	m.text = text
	m.offset = 0
	m.dir = 1
	m.holdUntil = now.Add(marqueeHoldDuration)
}

func (m *marqueeState) view() string {
	return fitMenuBar(m.text, m.offset, m.cells())
}

func (m *marqueeState) advance(now time.Time, playing bool) bool {
	width := displayWidth(m.text)
	slot := m.cells()
	if width <= slot {
		if m.offset == 0 {
			return false
		}
		m.offset = 0
		m.dir = 1
		return true
	}
	if m.reduceMotion {
		if m.offset == 0 {
			return false
		}
		m.offset = 0
		m.dir = 1
		return true
	}
	if !playing {
		return false
	}
	if now.Before(m.holdUntil) {
		return false
	}

	maxOffset := width - slot
	m.offset += m.dir
	if m.offset >= maxOffset {
		m.offset = maxOffset
		m.dir = -1
		m.holdUntil = now.Add(marqueeHoldDuration)
	} else if m.offset <= 0 {
		m.offset = 0
		m.dir = 1
		m.holdUntil = now.Add(marqueeHoldDuration)
	}
	return true
}

func normalizeSlotCells(slotCells int) int {
	if slotCells <= 0 {
		return defaultMenuBarSlotCells
	}
	minCells := minSlotWidth * 2
	maxCells := maxSlotWidth * 2
	if slotCells < minCells {
		return minCells
	}
	if slotCells > maxCells {
		return maxCells
	}
	return slotCells
}

func fitMenuBar(text string, offset, slotCells int) string {
	text = strings.TrimSpace(text)
	slotCells = normalizeSlotCells(slotCells)
	width := displayWidth(text)
	if width <= slotCells {
		return centerInSlot(text, slotCells)
	}

	maxOffset := width - slotCells
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	return sliceByCells(text, offset, slotCells)
}

func centerInSlot(text string, slotCells int) string {
	slotCells = normalizeSlotCells(slotCells)
	width := displayWidth(text)
	if width >= slotCells {
		return sliceByCells(text, 0, slotCells)
	}
	left := (slotCells - width) / 2
	right := slotCells - width - left
	return padCells(left) + text + padCells(right)
}

func padCells(n int) string {
	if n <= 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(n)
	for n >= 2 {
		b.WriteRune(menuBarPadFull)
		n -= 2
	}
	if n == 1 {
		b.WriteRune(menuBarPadHalf)
	}
	return b.String()
}

func sliceByCells(text string, start, max int) string {
	if max <= 0 {
		return ""
	}

	skipped := 0
	taken := 0
	var b strings.Builder
	for _, r := range text {
		w := runeCells(r)
		if w <= 0 {
			continue
		}
		if skipped < start {
			skipped += w
			continue
		}
		if taken+w > max {
			break
		}
		b.WriteRune(r)
		taken += w
	}
	if taken < max {
		b.WriteString(padCells(max - taken))
	}
	return b.String()
}

func displayWidth(text string) int {
	width := 0
	for _, r := range text {
		width += runeCells(r)
	}
	return width
}

func runeCells(r rune) int {
	switch r {
	case menuBarPadFull:
		return 2
	case menuBarPadHalf, '\u2800', '\u00A0':
		return 1
	case '\uFE0F', '\u200D', '\uFE0E':
		return 0
	}

	if r < 0x7F {
		if r < 0x20 {
			return 0
		}
		return 1
	}

	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
		return 0
	}

	switch {
	case r >= 0x1100 && r <= 0x115F,
		r >= 0x2329 && r <= 0x232A,
		r >= 0x2E80 && r <= 0xA4CF && r != 0x303F,
		r >= 0xAC00 && r <= 0xD7A3,
		r >= 0xF900 && r <= 0xFAFF,
		r >= 0xFE10 && r <= 0xFE19,
		r >= 0xFE30 && r <= 0xFE6F,
		r >= 0xFF00 && r <= 0xFF60,
		r >= 0xFFE0 && r <= 0xFFE6,
		r >= 0x1F300 && r <= 0x1FAFF,
		r >= 0x20000 && r <= 0x3FFFD:
		return 2
	default:
		return 1
	}
}

func systemReduceMotionEnabled() bool {
	out, err := exec.Command("defaults", "read", "com.apple.universalaccess", "reduceMotion").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "1"
}
