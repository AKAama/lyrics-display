package main

import (
	"context"
	"testing"
	"time"
)

func TestParseTTMLLineTimed(t *testing.T) {
	raw := `<tt xmlns="http://www.w3.org/ns/ttml" itunes:timing="Line"><body>
<p begin="21.211" end="26.871" itunes:key="L1">已经为了变得更好去掉锋芒</p>
<p begin="00:00:29.294" end="35.157"><span begin="29.294">一不小心</span><span>成了你的倾诉对象</span></p>
</body></tt>`
	lines := parseTTML(raw)
	if len(lines) != 2 {
		t.Fatalf("len=%d", len(lines))
	}
	if lines[0].Text != "已经为了变得更好去掉锋芒" {
		t.Fatalf("line0=%q", lines[0].Text)
	}
	if lines[0].At != 21211*time.Millisecond {
		t.Fatalf("at0=%s", lines[0].At)
	}
	if lines[1].Text != "一不小心成了你的倾诉对象" {
		t.Fatalf("line1=%q", lines[1].Text)
	}
	if lines[1].At != 29294*time.Millisecond {
		t.Fatalf("at1=%s", lines[1].At)
	}
}

func TestParseTTMLClockFormats(t *testing.T) {
	cases := map[string]time.Duration{
		"12.5":       12500 * time.Millisecond,
		"00:12.160":  12160 * time.Millisecond,
		"00:00:01.5": 1500 * time.Millisecond,
		"1:02":       62 * time.Second,
		"3.0s":       3 * time.Second,
	}
	for raw, want := range cases {
		got, err := parseTTMLClock(raw)
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		if got != want {
			t.Fatalf("%s: got %s want %s", raw, got, want)
		}
	}
}

func TestAppleSongIDFromURL(t *testing.T) {
	url := "https://se2.itunes.apple.com/WebObjects/MZStoreElements2.woa/wa/ttmlLyrics?id=200476335&l=zh_CN&itre=0"
	if got := appleSongIDFromURL(url); got != "200476335" {
		t.Fatalf("got %q", got)
	}
}

func TestFetchAppleCatalogLyricsFromLocalCache(t *testing.T) {
	np := nowPlaying{Track: "忽然之间", Artist: "莫文蔚"}
	doc, err := fetchAppleCatalogLyrics(context.Background(), np)
	if err != nil {
		t.Skipf("local Apple Music cache not usable: %v", err)
	}
	if doc.SourceKind != lyricSourceApple {
		t.Fatalf("kind=%q", doc.SourceKind)
	}
	if len(doc.Lines) == 0 {
		t.Fatal("expected timed apple lyrics")
	}
}

func TestAppleMatchScorePrefersSameTitle(t *testing.T) {
	np := nowPlaying{Track: "忽然之间", Artist: "莫文蔚", Duration: 200 * time.Second}
	hit := itunesSong{Name: "忽然之间", Artist: "莫文蔚", Duration: 201 * time.Second}
	miss := itunesSong{Name: "戒烟", Artist: "李荣浩", Duration: 240 * time.Second}
	if appleMatchScore(np, hit) < appleLyricsMatchThreshold {
		t.Fatalf("expected hit to match, score=%d", appleMatchScore(np, hit))
	}
	if appleMatchScore(np, miss) >= appleLyricsMatchThreshold {
		t.Fatalf("expected miss to stay below threshold, score=%d", appleMatchScore(np, miss))
	}
}
