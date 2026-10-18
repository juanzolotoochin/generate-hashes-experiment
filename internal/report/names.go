package report

import (
	"strings"

	"github.com/juanzolotoochin/generate-hashes-experiment/internal/bench"
)

// displayNames are the reports' names for tools whose harness IDs (used in
// -tools and results.json) say less: custom and custom-cquery are the custom
// Bazel's query and cquery with --experimental_generate_hashes.
var displayNames = map[string]string{
	"custom":        "generate_hashes (query)",
	"custom-cquery": "generate_hashes (cquery)",
}

// displayName maps a tool ID, with an optional @mode suffix, to its report name.
func displayName(t string) string {
	base, suffix, hasSuffix := strings.Cut(t, "@")
	n, ok := displayNames[base]
	if !ok {
		return t
	}
	if hasSuffix {
		return n + "@" + suffix
	}
	return n
}

// renamed returns a copy of r with every tool ID replaced by its display name.
func renamed(r *bench.Results) *bench.Results {
	c := *r
	c.Tools = nil
	for _, t := range r.Tools {
		c.Tools = append(c.Tools, displayName(t))
	}
	c.Scenarios = make([]bench.ScenarioResult, len(r.Scenarios))
	for i, sc := range r.Scenarios {
		sc.Tools = append([]bench.ToolScore(nil), sc.Tools...)
		for j := range sc.Tools {
			sc.Tools[j].Tool = displayName(sc.Tools[j].Tool)
		}
		c.Scenarios[i] = sc
	}
	c.NoChange = renameKeys(r.NoChange)
	c.Setup = renameKeys(r.Setup)
	c.BaseHashes = renameKeys(r.BaseHashes)
	c.Determinism = append([]bench.DetResult(nil), r.Determinism...)
	for i := range c.Determinism {
		c.Determinism[i].Tool = displayName(c.Determinism[i].Tool)
	}
	return &c
}

func renameKeys[V any](m map[string]V) map[string]V {
	if m == nil {
		return nil
	}
	out := make(map[string]V, len(m))
	for k, v := range m {
		out[displayName(k)] = v
	}
	return out
}
