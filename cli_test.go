package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestHandleCLIServiceFlagStartsApp(t *testing.T) {
	var buf bytes.Buffer
	result := handleCLI([]string{"--service"}, &buf, &configStore{path: "/tmp/lyrics-display-test.json"}, defaultConfig())
	if result.Handled {
		t.Fatal("--service should start the app, not be handled as a CLI command")
	}
	if !result.Service {
		t.Fatal("expected Service=true")
	}
}

func TestHandleCLIHelpIsHandled(t *testing.T) {
	var buf bytes.Buffer
	result := handleCLI([]string{"--help"}, &buf, &configStore{path: "/tmp/lyrics-display-test.json"}, defaultConfig())
	if !result.Handled {
		t.Fatal("help should be handled")
	}
	if !strings.Contains(buf.String(), "Start the menu bar app") {
		t.Fatalf("unexpected help output: %s", buf.String())
	}
}
