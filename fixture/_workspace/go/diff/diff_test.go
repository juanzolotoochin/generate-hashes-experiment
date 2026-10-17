package diff

import (
	"strings"
	"testing"
)

func TestLines(t *testing.T) {
	d, err := Lines([]string{"x\n"}, []string{"y\n"})
	if err != nil || !strings.Contains(d, "-x") || !strings.Contains(d, "+y") {
		t.Fatalf("Lines() = %q, %v", d, err)
	}
}
