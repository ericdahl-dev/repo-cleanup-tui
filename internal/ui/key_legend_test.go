package ui

import (
	"strings"
	"testing"
)

func TestKeyLegendPageShowsSpacedBrackets(t *testing.T) {
	got := keyLegend("[ ]", "page")
	if !strings.Contains(got, "[") || !strings.Contains(got, "]") || !strings.Contains(got, "page") {
		t.Fatalf("expected [ ] page legend, got %q", got)
	}
}

func TestKeyLegendFilter(t *testing.T) {
	got := keyLegend("f", "filter")
	if !strings.Contains(got, "filter") {
		t.Fatalf("missing filter label: %q", got)
	}
}
