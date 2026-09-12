package main

import "testing"

func TestParseAppleScriptNumber(t *testing.T) {
	tests := map[string]float64{
		"130.516006469727": 130.516006469727,
		"130,516006469727": 130.516006469727,
		"0":                0,
	}

	for input, want := range tests {
		got, err := parseAppleScriptNumber(input)
		if err != nil {
			t.Fatalf("parseAppleScriptNumber(%q): %v", input, err)
		}
		if got != want {
			t.Errorf("parseAppleScriptNumber(%q) = %v, want %v", input, got, want)
		}
	}
}
