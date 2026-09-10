package main

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestFitMenuBarAlwaysFillsSlot(t *testing.T) {
	cases := []string{
		"",
		"嗨",
		"你好",
		"你的回话凌乱着 在这个时刻",
		"I just wanna tell you that I'm nothing without you baby",
		strings.Repeat("字", 40),
		"ABC",
	}
	for _, text := range cases {
		got := fitMenuBar(text, 0, defaultMenuBarSlotCells)
		if displayWidth(got) != defaultMenuBarSlotCells {
			t.Fatalf("width of %q is %d, want %d (view=%q)", text, displayWidth(got), defaultMenuBarSlotCells, got)
		}
	}
}

func TestCenterInSlotPadsBothSides(t *testing.T) {
	got := centerInSlot("嗨", defaultMenuBarSlotCells)
	if displayWidth(got) != defaultMenuBarSlotCells {
		t.Fatalf("width = %d", displayWidth(got))
	}
	if !strings.Contains(got, "嗨") {
		t.Fatalf("missing text: %q", got)
	}
	prefix, _, _ := strings.Cut(got, "嗨")
	if displayWidth(prefix) != (defaultMenuBarSlotCells-2)/2 {
		t.Fatalf("left pad cells = %d, view=%q", displayWidth(prefix), got)
	}
}

func TestSliceByCellsScrollsAcrossCJK(t *testing.T) {
	text := "一二三四五六七八九十"
	first := sliceByCells(text, 0, 8)
	if first != "一二三四" {
		t.Fatalf("first window = %q", first)
	}
	second := sliceByCells(text, 2, 8)
	if second != "二三四五" {
		t.Fatalf("second window = %q", second)
	}
}

func TestMarqueeDoesNotMoveShortText(t *testing.T) {
	m := marqueeState{dir: 1}
	now := time.Now()
	m.setText("短句", now)
	if m.advance(now.Add(3*time.Second), true) {
		t.Fatal("short text should not scroll")
	}
	if displayWidth(m.view()) != defaultMenuBarSlotCells {
		t.Fatalf("view width = %d", displayWidth(m.view()))
	}
}

func TestMarqueePausesThenPongPongs(t *testing.T) {
	m := marqueeState{dir: 1}
	now := time.Now()
	m.setText(strings.Repeat("字", 40), now)

	if m.advance(now.Add(100*time.Millisecond), true) {
		t.Fatal("should hold at the start")
	}
	if m.offset != 0 {
		t.Fatalf("offset = %d", m.offset)
	}

	if !m.advance(now.Add(marqueeHoldDuration+time.Millisecond), true) {
		t.Fatal("expected to start scrolling after hold")
	}
	if m.offset != 1 {
		t.Fatalf("offset = %d, want 1", m.offset)
	}

	end := now.Add(time.Minute)
	for i := 0; i < 200 && m.dir == 1; i++ {
		m.advance(end, true)
		end = end.Add(marqueeTickInterval)
	}
	if m.dir != -1 {
		t.Fatal("expected to reverse at the end")
	}
	maxOffset := displayWidth(m.text) - defaultMenuBarSlotCells
	if m.offset != maxOffset {
		t.Fatalf("end offset = %d, want %d", m.offset, maxOffset)
	}
}

func TestMarqueeResetsWhenLineChanges(t *testing.T) {
	m := marqueeState{dir: 1}
	now := time.Now()
	m.setText(strings.Repeat("甲", 40), now)
	m.offset = 12
	m.setText(strings.Repeat("乙", 40), now)
	if m.offset != 0 || m.dir != 1 {
		t.Fatalf("offset=%d dir=%d", m.offset, m.dir)
	}
}

func TestMarqueeFreezesWhilePaused(t *testing.T) {
	m := marqueeState{dir: 1, offset: 8, text: strings.Repeat("字", 40)}
	if m.advance(time.Now().Add(time.Second), false) {
		t.Fatal("paused marquee should freeze")
	}
	if m.offset != 8 {
		t.Fatalf("offset changed to %d", m.offset)
	}
}

func TestDisplayWidthCountsCJKAsTwo(t *testing.T) {
	if displayWidth("A") != 1 {
		t.Fatal("ascii")
	}
	if displayWidth("字") != 2 {
		t.Fatal("cjk")
	}
	if displayWidth("字A") != 3 {
		t.Fatal("mixed")
	}
	if utf8.RuneCountInString(padCells(3)) != 2 {
		t.Fatalf("pad 3 should be one full + one half, got %q", padCells(3))
	}
	if displayWidth(padCells(3)) != 3 {
		t.Fatalf("pad width = %d", displayWidth(padCells(3)))
	}
}

func TestFitMenuBarHonorsCustomSlotWidth(t *testing.T) {
	narrow := fitMenuBar("你好世界", 0, 16)
	wide := fitMenuBar("你好世界", 0, 40)
	if displayWidth(narrow) != 16 {
		t.Fatalf("narrow width = %d", displayWidth(narrow))
	}
	if displayWidth(wide) != 40 {
		t.Fatalf("wide width = %d", displayWidth(wide))
	}
}

func TestNormalizeSlotWidth(t *testing.T) {
	cfg := config{SlotWidth: 0}
	cfg.normalize()
	if cfg.SlotWidth != defaultSlotWidth {
		t.Fatalf("default = %d", cfg.SlotWidth)
	}
	cfg.SlotWidth = 2
	cfg.normalize()
	if cfg.SlotWidth != minSlotWidth {
		t.Fatalf("min = %d", cfg.SlotWidth)
	}
	cfg.SlotWidth = 99
	cfg.normalize()
	if cfg.SlotWidth != maxSlotWidth {
		t.Fatalf("max = %d", cfg.SlotWidth)
	}
	if cfg.slotCells() != maxSlotWidth*2 {
		t.Fatalf("cells = %d", cfg.slotCells())
	}
}
