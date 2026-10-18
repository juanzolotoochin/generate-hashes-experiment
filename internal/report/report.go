// Package report renders benchmark results as HTML and Markdown.
package report

import (
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/juanzolotoochin/generate-hashes-experiment/internal/bench"
	"github.com/juanzolotoochin/generate-hashes-experiment/internal/netmeter"
)

//go:embed report.html.tmpl
var htmlTmpl string

// ToolSummary aggregates one tool across scenarios.
type ToolSummary struct {
	Tool             string
	Scored           int
	Exact            int
	WithMisses       int
	WithExtras       int
	Missed           int
	Extra            int
	TP               int
	Errors           int
	Recall           string
	Precision        string
	NoChangeMedian   string
	ScenarioMedian   string
	CategoryMedians  map[string]string
	MissedScenarios  []string
	ErroredScenarios []string
	SetupSeconds     string
	SetupDownload    string
	ChangeDownload   string // total over all non-control scenarios
}

type view struct {
	R           *bench.Results
	Summaries   []ToolSummary
	Categories  []string
	Consistency []Consistency
}

// Consistency compares one tool's hashes of the same base revision in two
// setups: with the remote repo contents cache off and on, or from another
// checkout with another output base.
type Consistency struct {
	Tool      string
	Against   string
	Compared  int
	Different []string
}

var consistencyChecks = []struct{ suffix, desc string }{
	{"@relocated", "another checkout and output base"},
	{"@remote", "remote repo contents cache on"},
}

func consistency(r *bench.Results) []Consistency {
	var out []Consistency
	for _, chk := range consistencyChecks {
		for _, t := range r.Tools {
			if c, ok := compareHashes(r, t, chk.suffix, chk.desc); ok {
				out = append(out, c)
			}
		}
	}
	return out
}

func compareHashes(r *bench.Results, t, suffix, desc string) (Consistency, bool) {
	a, b := r.BaseHashes[t], r.BaseHashes[t+suffix]
	if a == nil || b == nil {
		return Consistency{}, false
	}
	c := Consistency{Tool: t, Against: desc}
	for l, h := range a {
		if h2, ok := b[l]; ok {
			c.Compared++
			if h != h2 {
				c.Different = append(c.Different, l)
			}
		}
	}
	sort.Strings(c.Different)
	return c, true
}

// Write renders report.html and summary.md into dir and returns their paths.
func Write(dir string, r *bench.Results) ([]string, error) {
	v := view{R: r, Summaries: summarize(r), Consistency: consistency(r)}
	seen := map[string]bool{}
	for _, s := range r.Scenarios {
		if !seen[s.Category] {
			seen[s.Category] = true
			v.Categories = append(v.Categories, s.Category)
		}
	}
	t, err := template.New("report").Funcs(template.FuncMap{
		"secs":  func(f float64) string { return fmt.Sprintf("%.1fs", f) },
		"join":  strings.Join,
		"short": shortList,
		"cell":  cellClass,
		"mb":    mb,
		"gb":    gb,
	}).Parse(htmlTmpl)
	if err != nil {
		return nil, err
	}
	htmlPath := filepath.Join(dir, "report.html")
	f, err := os.Create(htmlPath)
	if err != nil {
		return nil, err
	}
	if err := t.Execute(f, v); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	mdPath := filepath.Join(dir, "summary.md")
	if err := os.WriteFile(mdPath, []byte(markdown(v)), 0o644); err != nil {
		return nil, err
	}
	return []string{htmlPath, mdPath, filepath.Join(dir, "results.json")}, nil
}

func summarize(r *bench.Results) []ToolSummary {
	var out []ToolSummary
	for _, tool := range r.Tools {
		s := ToolSummary{Tool: tool, CategoryMedians: map[string]string{}}
		var times []float64
		var dl netmeter.Counts
		measured := false
		cat := map[string][]float64{}
		for _, sc := range r.Scenarios {
			for _, ts := range sc.Tools {
				if ts.Tool != tool {
					continue
				}
				if ts.Outcome.Err != "" {
					s.Errors++
					s.ErroredScenarios = append(s.ErroredScenarios, sc.Name)
					continue
				}
				if sc.Name != "control" {
					if d := ts.Outcome.Download; d != nil {
						measured = true
						dl.Internet += d.Internet
						dl.Remote += d.Remote
					}
					times = append(times, ts.Outcome.Duration.Seconds())
					cat[sc.Category] = append(cat[sc.Category], ts.Outcome.Duration.Seconds())
				}
				if !sc.HasOracle {
					continue
				}
				s.Scored++
				s.TP += ts.TP
				s.Missed += ts.FN
				s.Extra += ts.FP
				if ts.FN > 0 {
					s.WithMisses++
					s.MissedScenarios = append(s.MissedScenarios, sc.Name)
				}
				if ts.FP > 0 {
					s.WithExtras++
				}
				if ts.FN == 0 && ts.FP == 0 {
					s.Exact++
				}
			}
		}
		s.Recall = ratio(s.TP, s.TP+s.Missed)
		s.Precision = ratio(s.TP, s.TP+s.Extra)
		s.NoChangeMedian = medianStr(r.NoChange[tool])
		s.ScenarioMedian = medianStr(times)
		s.ChangeDownload = "–"
		if measured {
			s.ChangeDownload = mb(&dl)
		}
		s.SetupSeconds, s.SetupDownload = "–", "–"
		if st, ok := r.Setup[tool]; ok {
			s.SetupSeconds = fmt.Sprintf("%.0fs", st.Seconds)
			s.SetupDownload = mb(&st.Download)
		}
		for c, ts := range cat {
			s.CategoryMedians[c] = medianStr(ts)
		}
		out = append(out, s)
	}
	return out
}

// mb formats downloaded bytes, with the remote-cache share when there is one.
func mb(c *netmeter.Counts) string {
	if c == nil {
		return "–"
	}
	s := fmt.Sprintf("%.1f MB", float64(c.Total())/1e6)
	if c.Remote > 0 {
		s += fmt.Sprintf(" (%.0f%% remote)", 100*float64(c.Remote)/float64(c.Total()))
	}
	return s
}

func gb(n int64) string {
	switch {
	case n == 0:
		return "0"
	case n < 1e9:
		return fmt.Sprintf("%.0f MB", float64(n)/1e6)
	}
	return fmt.Sprintf("%.1f GB", float64(n)/1e9)
}

func ratio(a, b int) string {
	if b == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%%", 100*float64(a)/float64(b))
}

func medianStr(xs []float64) string {
	if len(xs) == 0 {
		return "–"
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	m := s[len(s)/2]
	if len(s)%2 == 0 {
		m = (s[len(s)/2-1] + s[len(s)/2]) / 2
	}
	return fmt.Sprintf("%.1fs", m)
}

func shortList(xs []string, n int) []string {
	if len(xs) <= n {
		return xs
	}
	return append(append([]string{}, xs[:n]...), fmt.Sprintf("… and %d more", len(xs)-n))
}

func cellClass(ts bench.ToolScore, hasOracle bool) string {
	switch {
	case ts.Outcome.Err != "":
		return "err"
	case !hasOracle:
		return ""
	case ts.FN > 0:
		return "bad"
	case ts.FP > 0:
		return "warn"
	default:
		return "ok"
	}
}

func markdown(v view) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# hashbench results\n\n")
	fmt.Fprintf(&b, "Stock Bazel %s · bazel-diff %s · target-determinator %s · scale %d · %d base targets\n\n",
		v.R.Meta["stock_bazel"], v.R.Meta["bazel_diff"], v.R.Meta["target_determinator"], v.R.Scale, v.R.BaseSize)
	b.WriteString("| Tool | Exact | Scenarios with misses | Missed targets | Over-selected targets | Recall | Precision | Warm no-change | Warm per change (median) |\n|---|---|---|---|---|---|---|---|---|\n")
	for _, s := range v.Summaries {
		fmt.Fprintf(&b, "| %s | %d/%d | %d | %d | %d | %s | %s | %s | %s |\n",
			s.Tool, s.Exact, s.Scored, s.WithMisses, s.Missed, s.Extra, s.Recall, s.Precision, s.NoChangeMedian, s.ScenarioMedian)
	}
	b.WriteString("\n## Downloads\n\nBytes each tool's Bazel received over the network (internet and remote cache), starting from its own empty caches.\n\n")
	b.WriteString("| Tool | Setup (base revision) | Setup time | All changes (total) |\n|---|---|---|---|\n")
	for _, s := range v.Summaries {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", s.Tool, s.SetupDownload, s.SetupSeconds, s.ChangeDownload)
	}
	b.WriteString("\n| Scenario | Category | Truth |")
	for _, t := range v.R.Tools {
		fmt.Fprintf(&b, " %s |", t)
	}
	b.WriteString("\n|---|---|---|" + strings.Repeat("---|", len(v.R.Tools)) + "\n")
	for _, sc := range v.R.Scenarios {
		truth := "–"
		if sc.HasOracle {
			truth = fmt.Sprint(len(sc.Truth))
		}
		fmt.Fprintf(&b, "| %s | %s | %s |", sc.Name, sc.Category, truth)
		for _, ts := range sc.Tools {
			switch {
			case ts.Outcome.Err != "":
				b.WriteString(" error |")
			case !sc.HasOracle:
				fmt.Fprintf(&b, " %d |", len(ts.Outcome.Labels))
			case ts.FN == 0 && ts.FP == 0:
				b.WriteString(" ✅ |")
			default:
				var parts []string
				if ts.FN > 0 {
					parts = append(parts, fmt.Sprintf("❌ %d missed", ts.FN))
				}
				if ts.FP > 0 {
					parts = append(parts, fmt.Sprintf("+%d extra", ts.FP))
				}
				fmt.Fprintf(&b, " %s |", strings.Join(parts, ", "))
			}
		}
		b.WriteString("\n")
	}
	if len(v.Consistency) > 0 {
		b.WriteString("\n## Consistency: same revision, different setups\n\n")
		b.WriteString("Each tool's hashes of the base revision in local mode, compared with its hashes from a second checkout at another path with fresh output bases and caches at other paths (as on another CI machine or worktree), and with `--experimental_remote_repo_contents_cache` (where external repos are served lazily and are not on disk). A correct tool gives identical hashes; any difference means comparing hash snapshots from different machines or runs gives wrong answers.\n\n")
		b.WriteString("| Tool | Compared with | Targets compared | Different hashes | Examples |\n|---|---|---|---|---|\n")
		for _, c := range v.Consistency {
			fmt.Fprintf(&b, "| %s | %s | %d | %d | %s |\n", c.Tool, c.Against, c.Compared, len(c.Different), strings.Join(shortList(c.Different, 4), ", "))
		}
	}
	if len(v.R.RepoSupport) > 0 {
		b.WriteString("\n## Repo contents cache support by module\n\n")
		b.WriteString("Every repository the tools fetched for the base revision. Only repositories whose rule declares them reproducible are cached (locally and by `--experimental_remote_repo_contents_cache`); the rest are fetched from the internet even with a remote cache. **Bold**: a direct `bazel_dep` of the fixture.\n\n")
		b.WriteString("| Module | Cacheable repos | Size | Not cacheable | Size | Not cacheable repos |\n|---|---|---|---|---|---|\n")
		for _, r := range v.R.RepoSupport {
			name := r.Module
			if r.Direct {
				name = "**" + name + "**"
			}
			fmt.Fprintf(&b, "| %s | %d | %s | %d | %s | %s |\n", name, r.Cached, gb(r.CachedBytes), r.NotCached, gb(r.NotCachedBytes),
				strings.Join(shortList(r.NotCachedRepos, 4), ", "))
		}
	}
	b.WriteString("\n### Downloads per scenario\n\n| Scenario |")
	for _, t := range v.R.Tools {
		fmt.Fprintf(&b, " %s |", t)
	}
	b.WriteString("\n|---|" + strings.Repeat("---|", len(v.R.Tools)) + "\n")
	for _, sc := range v.R.Scenarios {
		fmt.Fprintf(&b, "| %s |", sc.Name)
		for _, ts := range sc.Tools {
			fmt.Fprintf(&b, " %s |", mb(ts.Outcome.Download))
		}
		b.WriteString("\n")
	}
	return b.String()
}
