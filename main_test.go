package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestPrintHelpDocumentsSubcommands(t *testing.T) {
	var buf bytes.Buffer
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	printHelp()
	_ = w.Close()
	os.Stdout = old
	_, _ = io.Copy(&buf, r)
	out := buf.String()
	for _, sub := range []string{"init", "scan", "tui", "--json", "--version"} {
		if !strings.Contains(out, sub) {
			t.Fatalf("help missing %q:\n%s", sub, out)
		}
	}
}

func TestPrintVersion(t *testing.T) {
	var buf bytes.Buffer
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	printVersion()
	_ = w.Close()
	os.Stdout = old
	_, _ = io.Copy(&buf, r)
	out := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(out, "repo-cleanup-tui ") {
		t.Fatalf("version line = %q", out)
	}
}

func TestIsVersionFlag(t *testing.T) {
	for _, flag := range []string{"--version", "-version"} {
		if !isVersionFlag(flag) {
			t.Fatalf("%q should be version flag", flag)
		}
	}
	if isVersionFlag("-v") || isVersionFlag("version") {
		t.Fatal("unexpected version flag match")
	}
}

func TestLooksLikePath(t *testing.T) {
	if looksLikePath("init") {
		t.Fatal("command mistaken for path")
	}
	if !looksLikePath("/tmp/repos") {
		t.Fatal("path not recognized")
	}
	if !looksLikePath(".") {
		t.Fatal("relative path not recognized")
	}
}
