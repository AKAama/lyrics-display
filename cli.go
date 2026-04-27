package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func handleCLI(args []string, stdout io.Writer, store *configStore, cfg config) bool {
	fs := flag.NewFlagSet("lyrics-display", flag.ContinueOnError)
	fs.SetOutput(stdout)

	showVersion := fs.Bool("version", false, "print version information")
	showHelp := fs.Bool("help", false, "show help")
	fs.BoolVar(showHelp, "h", false, "show help")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stdout)
		printUsage(stdout)
		return true
	}

	if *showHelp {
		printUsage(stdout)
		return true
	}

	if *showVersion {
		fmt.Fprintln(stdout, versionString())
		return true
	}

	if fs.NArg() == 0 {
		return false
	}

	switch fs.Arg(0) {
	case "version":
		fmt.Fprintln(stdout, versionString())
		return true
	case "status":
		return handleStatus(stdout, store, cfg)
	case "config":
		return handleConfigCommand(stdout, store, cfg, fs.Args()[1:])
	case "offset":
		return handleOffsetCommand(stdout, store, cfg, fs.Args()[1:])
	}

	return false
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `lyrics-display

Apple Music menu bar lyrics for macOS.

Usage:
  lyrics-display                                        Start the menu bar app
  lyrics-display --help                                 Show this help text
  lyrics-display --version                              Print version information
  lyrics-display status                                 Show current config and runtime hints
  lyrics-display config show                            Print current config JSON
  lyrics-display config path                            Print config file path
  lyrics-display config init                            Create the default config file
  lyrics-display config set emoji on|off                Enable or disable the menu bar emoji
  lyrics-display config set emoji-char "♪"             Set the emoji or prefix symbol
  lyrics-display config set offset-ms 450               Set lyric sync offset in milliseconds
  lyrics-display offset +100                            Delay lyric matching by 100ms
  lyrics-display offset -100                            Advance lyric matching by 100ms
  lyrics-display offset set 350                         Set lyric offset directly
`)
}

func versionString() string {
	parts := []string{version}
	if commit != "" {
		parts = append(parts, "commit="+commit)
	}
	if buildDate != "" {
		parts = append(parts, "built="+buildDate)
	}
	return strings.Join(parts, " ")
}

func handleStatus(stdout io.Writer, store *configStore, cfg config) bool {
	cfg.normalize()
	fmt.Fprintf(stdout, "version: %s\n", versionString())
	fmt.Fprintf(stdout, "config: %s\n", store.pathString())
	fmt.Fprintf(stdout, "emoji: %t (%s)\n", cfg.ShowEmoji, cfg.Emoji)
	fmt.Fprintf(stdout, "offset_ms: %d\n", cfg.OffsetMS)
	fmt.Fprintf(stdout, "music_running: %t\n", musicAppRunning(context.Background()))
	fmt.Fprintln(stdout, "note: menu bar text color and font are controlled by macOS and are not configurable in this build.")
	return true
}

func handleConfigCommand(stdout io.Writer, store *configStore, cfg config, args []string) bool {
	if len(args) == 0 {
		printUsage(stdout)
		return true
	}

	switch args[0] {
	case "show":
		cfg.normalize()
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			fmt.Fprintf(stdout, "marshal config: %v\n", err)
			return true
		}
		fmt.Fprintf(stdout, "%s\n", data)
	case "path":
		fmt.Fprintln(stdout, store.pathString())
	case "init":
		if err := store.save(cfg); err != nil {
			fmt.Fprintf(stdout, "save config: %v\n", err)
			return true
		}
		fmt.Fprintf(stdout, "initialized config at %s\n", store.pathString())
	case "set":
		if len(args) < 3 {
			fmt.Fprintln(stdout, "usage: lyrics-display config set <emoji|emoji-char|offset-ms> <value>")
			return true
		}
		switch args[1] {
		case "emoji":
			value, ok := parseOnOff(args[2])
			if !ok {
				fmt.Fprintln(stdout, "emoji expects on or off")
				return true
			}
			cfg.ShowEmoji = value
		case "emoji-char":
			cfg.Emoji = args[2]
		case "offset-ms":
			value, err := strconv.Atoi(args[2])
			if err != nil {
				fmt.Fprintln(stdout, "offset-ms expects an integer")
				return true
			}
			cfg.OffsetMS = value
		default:
			fmt.Fprintln(stdout, "supported keys: emoji, emoji-char, offset-ms")
			return true
		}

		if err := store.save(cfg); err != nil {
			fmt.Fprintf(stdout, "save config: %v\n", err)
			return true
		}
		fmt.Fprintf(stdout, "updated config at %s\n", store.pathString())
	default:
		printUsage(stdout)
	}
	return true
}

func handleOffsetCommand(stdout io.Writer, store *configStore, cfg config, args []string) bool {
	if len(args) == 0 {
		fmt.Fprintln(stdout, "usage: lyrics-display offset [+100|-100|set 350]")
		return true
	}

	switch args[0] {
	case "set":
		if len(args) < 2 {
			fmt.Fprintln(stdout, "usage: lyrics-display offset set <milliseconds>")
			return true
		}
		value, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Fprintln(stdout, "offset set expects an integer")
			return true
		}
		cfg.OffsetMS = value
	default:
		delta, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Fprintln(stdout, "offset expects +N, -N, or set N")
			return true
		}
		cfg.OffsetMS += delta
	}

	if err := store.save(cfg); err != nil {
		fmt.Fprintf(stdout, "save config: %v\n", err)
		return true
	}

	updated, err := store.load()
	if err != nil {
		fmt.Fprintf(stdout, "reload config: %v\n", err)
		return true
	}
	fmt.Fprintf(stdout, "offset_ms: %d\n", updated.OffsetMS)
	return true
}

func parseOnOff(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "on", "true", "1":
		return true, true
	case "off", "false", "0":
		return false, true
	default:
		return false, false
	}
}
