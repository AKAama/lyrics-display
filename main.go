package main

import (
	"errors"
	"log"
	"os"
	"time"
)

const (
	menuBarMaxRunes  = 28
	pollInterval     = 500 * time.Millisecond
	defaultOffset    = 350 * time.Millisecond
	requestTimeout   = 6 * time.Second
	fieldSeparator   = "\x1f"
	appName          = "lyrics-display"
	defaultEmoji     = "♪"
	defaultSlotWidth = 18
	minSlotWidth     = 8
	maxSlotWidth     = 40
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

	result := handleCLI(os.Args[1:], os.Stdout, store, cfg)
	if result.Handled {
		return
	}

	if err := acquireInstanceLock(store); err != nil {
		if errors.Is(err, errAlreadyRunning) {
			notifyAlreadyRunning()
			log.Fatalf("lyrics-display is already running")
		}
		log.Fatalf("acquire instance lock: %v", err)
	}

	startMenuBarApp(store, cfg, detectServiceManaged(result.Service))
}
