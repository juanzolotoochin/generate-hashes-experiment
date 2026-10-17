package greet

import (
	"strings"
	"testing"
)

func TestGreeting(t *testing.T) {
	if got := Greeting("bazel"); !strings.HasPrefix(got, "hello, bazel") {
		t.Fatalf("unexpected greeting %q", got)
	}
}
