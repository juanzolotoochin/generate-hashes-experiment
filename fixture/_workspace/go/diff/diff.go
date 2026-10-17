// Package diff wraps go-difflib.
package diff

import "github.com/pmezard/go-difflib/difflib"

// Lines returns a unified diff of two line slices.
func Lines(a, b []string) (string, error) {
	return difflib.GetUnifiedDiffString(difflib.UnifiedDiff{A: a, B: b, FromFile: "a", ToFile: "b", Context: 1})
}
