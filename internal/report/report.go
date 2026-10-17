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
}

type view struct {
	R          *bench.Results
	Summaries  []ToolSummary
	Categories []string
}

// Write renders report.html and summary.md into dir and returns their paths.
func Write(dir string, r *bench.Results) ([]string, error) {
	v := view{R: r, Summaries: summarize(r)}
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
		for c, ts := range cat {
			s.CategoryMedians[c] = medianStr(ts)
		}
		out = append(out, s)
	}
	return out
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
	return b.String()
}
