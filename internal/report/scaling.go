package report

import (
	_ "embed"
	"fmt"
	"html/template"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/juanzolotoochin/generate-hashes-experiment/internal/bench"
)

//go:embed scaling.html.tmpl
var scalingTmpl string

// Categorical slots in fixed order (never cycled); tools beyond three fold
// into the table only.
var seriesVars = []string{"--series-1", "--series-2", "--series-3"}

type scalePoint struct {
	Targets  int
	Scale    int
	NoChange float64 // median seconds, -1 when missing
	Change   float64 // median seconds over non-control scenarios, -1 when missing
	Errors   int
}

type scaleSeries struct {
	Tool   string
	Var    string
	Points []scalePoint
}

type chartPoint struct {
	X, Y   float64
	Label  string
	Series string
}

type chart struct {
	Title, Subtitle string
	Width, Height   int
	Lines           []chartLine
	XTicks, YTicks  []chartTick
	PlotLeft        float64
	PlotRight       float64
	PlotTop         float64
	PlotBottom      float64
}

type chartLine struct {
	Tool, Var  string
	Path       string
	Points     []chartPoint
	EndX, EndY float64
	EndLabel   string
}

type chartTick struct {
	Pos   float64
	Label string
}

type scenarioRow struct {
	Name    string
	Targets int
	Cells   []string
}

type scalingView struct {
	Results   []*bench.Results
	Series    []scaleSeries
	Charts    []chart
	Tools     []string
	Scenarios []scenarioRow
	Meta      map[string]string
}

// WriteScaling renders scaling.html and scaling.md for a sweep of runs.
func WriteScaling(dir string, runs []*bench.Results) ([]string, error) {
	if len(runs) == 0 {
		return nil, fmt.Errorf("no runs")
	}
	named := make([]*bench.Results, len(runs))
	for i, r := range runs {
		named[i] = renamed(r)
	}
	runs = named
	sort.Slice(runs, func(i, j int) bool { return runs[i].Scale < runs[j].Scale })
	v := scalingView{Results: runs, Tools: runs[0].Tools, Meta: runs[0].Meta}
	for i, tool := range v.Tools {
		s := scaleSeries{Tool: tool}
		if i < len(seriesVars) {
			s.Var = seriesVars[i]
		}
		for _, r := range runs {
			p := scalePoint{Targets: r.RuleTargets, Scale: r.Scale, NoChange: median(r.NoChange[tool]), Change: -1}
			var times []float64
			for _, sc := range r.Scenarios {
				for _, ts := range sc.Tools {
					if ts.Tool != tool {
						continue
					}
					if ts.Outcome.Err != "" {
						p.Errors++
					} else if sc.Name != "control" {
						times = append(times, ts.Outcome.Duration.Seconds())
					}
				}
			}
			p.Change = median(times)
			s.Points = append(s.Points, p)
		}
		v.Series = append(v.Series, s)
	}
	v.Charts = []chart{
		buildChart("No change", "Median of repeated warm runs on an empty commit", v.Series, func(p scalePoint) float64 { return p.NoChange }),
		buildChart("Per change", "Median across the representative change scenarios", v.Series, func(p scalePoint) float64 { return p.Change }),
	}
	// Per-scenario detail, one row per (scenario, scale).
	for _, r := range runs {
		for _, sc := range r.Scenarios {
			row := scenarioRow{Name: sc.Name, Targets: r.RuleTargets}
			for _, tool := range v.Tools {
				cell := "–"
				for _, ts := range sc.Tools {
					if ts.Tool == tool {
						if ts.Outcome.Err != "" {
							cell = "error"
						} else {
							cell = fmt.Sprintf("%.1fs", ts.Outcome.Duration.Seconds())
						}
					}
				}
				row.Cells = append(row.Cells, cell)
			}
			v.Scenarios = append(v.Scenarios, row)
		}
	}

	t, err := template.New("scaling").Funcs(template.FuncMap{
		"secs": fmtSecs,
		"num":  func(n int) string { return groupDigits(n) },
		"cssVar": func(name string) template.CSS {
			if name == "" {
				return "var(--text-secondary)"
			}
			return template.CSS("var(" + name + ")")
		},
	}).Parse(scalingTmpl)
	if err != nil {
		return nil, err
	}
	htmlPath := filepath.Join(dir, "scaling.html")
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
	mdPath := filepath.Join(dir, "scaling.md")
	if err := os.WriteFile(mdPath, []byte(scalingMarkdown(v)), 0o644); err != nil {
		return nil, err
	}
	return []string{htmlPath, mdPath}, nil
}

func buildChart(title, subtitle string, series []scaleSeries, y func(scalePoint) float64) chart {
	c := chart{Title: title, Subtitle: subtitle, Width: 560, Height: 320, PlotLeft: 56, PlotRight: 470, PlotTop: 16, PlotBottom: 280}
	maxX, maxY := 1.0, 0.0
	for _, s := range series {
		if s.Var == "" {
			continue
		}
		for _, p := range s.Points {
			maxX = math.Max(maxX, float64(p.Targets))
			if v := y(p); v >= 0 {
				maxY = math.Max(maxY, v)
			}
		}
	}
	yStep := niceStep(maxY / 4)
	yMax := yStep * math.Ceil(maxY/yStep)
	if yMax == 0 {
		yMax = 1
	}
	xStep := niceStep(maxX / 4)
	xMax := xStep * math.Ceil(maxX/xStep)
	sx := func(v float64) float64 { return c.PlotLeft + v/xMax*(c.PlotRight-c.PlotLeft) }
	sy := func(v float64) float64 { return c.PlotBottom - v/yMax*(c.PlotBottom-c.PlotTop) }
	for v := 0.0; v <= yMax+1e-9; v += yStep {
		c.YTicks = append(c.YTicks, chartTick{sy(v), fmtSecs(v)})
	}
	for v := 0.0; v <= xMax+1e-9; v += xStep {
		c.XTicks = append(c.XTicks, chartTick{sx(v), groupDigits(int(v))})
	}
	for _, s := range series {
		if s.Var == "" {
			continue
		}
		l := chartLine{Tool: s.Tool, Var: s.Var}
		var b strings.Builder
		for _, p := range s.Points {
			v := y(p)
			if v < 0 {
				continue
			}
			x, yy := sx(float64(p.Targets)), sy(v)
			if b.Len() == 0 {
				fmt.Fprintf(&b, "M%.1f %.1f", x, yy)
			} else {
				fmt.Fprintf(&b, " L%.1f %.1f", x, yy)
			}
			l.Points = append(l.Points, chartPoint{X: x, Y: yy, Series: s.Tool,
				Label: fmt.Sprintf("%s · %s targets · %s", s.Tool, groupDigits(p.Targets), fmtSecs(v))})
			l.EndX, l.EndY, l.EndLabel = x, yy, fmtSecs(v)
		}
		l.Path = b.String()
		c.Lines = append(c.Lines, l)
	}
	spreadEndLabels(c.Lines)
	return c
}

// spreadEndLabels nudges direct labels apart so they don't overlap.
func spreadEndLabels(lines []chartLine) {
	idx := make([]int, len(lines))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool { return lines[idx[a]].EndY < lines[idx[b]].EndY })
	for k := 1; k < len(idx); k++ {
		prev, cur := &lines[idx[k-1]], &lines[idx[k]]
		if cur.EndY-prev.EndY < 14 {
			cur.EndY = prev.EndY + 14
		}
	}
}

func niceStep(raw float64) float64 {
	if raw <= 0 {
		return 1
	}
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	for _, m := range []float64{1, 2, 2.5, 5, 10} {
		if raw <= m*mag {
			return m * mag
		}
	}
	return 10 * mag
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return -1
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	if len(s)%2 == 1 {
		return s[len(s)/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}

func fmtSecs(v float64) string {
	switch {
	case v < 0:
		return "–"
	case v < 10:
		return fmt.Sprintf("%.1fs", v)
	default:
		return fmt.Sprintf("%.0fs", v)
	}
}

func groupDigits(n int) string {
	s := fmt.Sprint(n)
	var out []byte
	for i := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, s[i])
	}
	return string(out)
}

func scalingMarkdown(v scalingView) string {
	var b strings.Builder
	b.WriteString("# hashbench scaling\n\n")
	b.WriteString("Warm wall-clock seconds to compute affected targets. *No change* is the median of repeated runs on an empty commit; *per change* is the median across the change scenarios.\n\n")
	b.WriteString("| Rule targets |")
	for _, t := range v.Tools {
		fmt.Fprintf(&b, " %s no change | %s per change |", t, t)
	}
	b.WriteString("\n|---|" + strings.Repeat("---|---|", len(v.Tools)) + "\n")
	for i, r := range v.Results {
		fmt.Fprintf(&b, "| %s |", groupDigits(r.RuleTargets))
		for _, s := range v.Series {
			p := s.Points[i]
			fmt.Fprintf(&b, " %s | %s |", fmtSecs(p.NoChange), fmtSecs(p.Change))
		}
		b.WriteString("\n")
	}
	return b.String()
}
