package main

import (
	"log"
	"os"
	"time"
)

const (
	menuBarMaxRunes = 28
	pollInterval    = 500 * time.Millisecond
	defaultOffset   = 350 * time.Millisecond
	requestTimeout  = 6 * time.Second
	fieldSeparator  = "\x1f"
	appName         = "lyrics-display"
	defaultEmoji    = "♪"
)

var (
	version   = "dev"
	commit    = ""
	buildDate = ""
)

func main() {
	store, err := newConfigStore()
	if err != nil {
		log.Fatalf("resolve config path: %v", err)
	}

	cfg, err := store.load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if handled := handleCLI(os.Args[1:], os.Stdout, store, cfg); handled {
		return
	}

	startMenuBarApp(store, cfg)
}
