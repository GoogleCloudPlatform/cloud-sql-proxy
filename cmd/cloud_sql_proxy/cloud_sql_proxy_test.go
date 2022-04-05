package main

import (
	"os"
	"strings"
	"testing"
)

func TestVersionStripsNewlint(t *testing.T) {
	v, err := os.ReadFile("version.txt")
	if err != nil {
		t.Fatalf("failed to read verion.txt: %v", err)
	}
	want := strings.TrimSpace(string(v))

	if got := semanticVersion(); got != want {
		t.Fatalf("want = %q, got = %q", want, got)
	}
}
