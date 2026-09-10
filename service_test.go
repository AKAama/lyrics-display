package main

import "testing"

func TestIsBrewXPCService(t *testing.T) {
	cases := map[string]bool{
		"homebrew.mxcl.lyrics-display":          true,
		"gui/501/homebrew.mxcl.lyrics-display":  true,
		"application.com.akaama.lyrics-display": false,
		"com.akaama.lyrics-display":             false,
		"":                                      false,
	}
	for name, want := range cases {
		if got := isBrewXPCService(name); got != want {
			t.Errorf("isBrewXPCService(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestParseLaunchdJobPID(t *testing.T) {
	output := `gui/501/homebrew.mxcl.lyrics-display = {
	active count = 1
	path = /Users/alex/Library/LaunchAgents/homebrew.mxcl.lyrics-display.plist
	type = LaunchAgent
	state = running

	program = /opt/homebrew/opt/lyrics-display/bin/lyrics-display
	arguments = {
		/opt/homebrew/opt/lyrics-display/bin/lyrics-display
		--service
	}

	pid = 4242
	last exit code = (never exited)
}`
	pid, ok := parseLaunchdJobPID(output)
	if !ok || pid != 4242 {
		t.Fatalf("pid=%d ok=%v, want 4242", pid, ok)
	}

	if _, ok := parseLaunchdJobPID("no pid here"); ok {
		t.Fatal("expected missing pid")
	}
}

func TestDetectServiceManagedHonorsFlag(t *testing.T) {
	if !detectServiceManaged(true) {
		t.Fatal("flag should force service mode")
	}
}

func TestSourceTitle(t *testing.T) {
	a := &app{}
	music := lyricDocument{SourceKind: lyricSourceMusic}
	if got := a.sourceTitle(music); got != "歌词源：Music 内置" {
		t.Fatalf("got %q", got)
	}
	plain := lyricDocument{SourceKind: lyricSourceMusic, Untimed: true}
	if got := a.sourceTitle(plain); got != "歌词源：Music 内置（无时间轴）" {
		t.Fatalf("got %q", got)
	}
	none := lyricDocument{}
	if got := a.sourceTitle(none); got != "歌词源：未命中，显示歌曲信息" {
		t.Fatalf("got %q", got)
	}
	retry := lyricDocument{FetchError: "eof"}
	if got := a.sourceTitle(retry); got != "歌词源：未命中，可重新搜索" {
		t.Fatalf("got %q", got)
	}
	lrclib := lyricDocument{SourceKind: lyricSourceLRCLIB, SourceID: 9, Candidates: []lyricCandidate{{ID: 9}, {ID: 8}}}
	if got := a.sourceTitle(lrclib); got != "歌词源：1/2 LRCLIB #9" {
		t.Fatalf("got %q", got)
	}
}
