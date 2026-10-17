package tools_test

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestSummary checks the jq summary counted at least one JSON file and that the
// fixture data file shipped in runfiles.
func TestSummary(t *testing.T) {
	data, err := os.ReadFile(os.Args[len(os.Args)-1])
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || n < 1 {
		t.Fatalf("summary = %q, want a positive count", data)
	}
	if _, err := os.Stat("tools/fixture.txt"); err != nil {
		t.Fatal(err)
	}
}
