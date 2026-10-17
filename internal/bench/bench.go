// Package bench runs the scenarios through every tool and the oracle.
package bench

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/juanzolotoochin/generate-hashes-experiment/fixture"
	"github.com/juanzolotoochin/generate-hashes-experiment/internal/oracle"
	"github.com/juanzolotoochin/generate-hashes-experiment/internal/scale"
	"github.com/juanzolotoochin/generate-hashes-experiment/internal/scenario"
	"github.com/juanzolotoochin/generate-hashes-experiment/internal/tools"
	"github.com/juanzolotoochin/generate-hashes-experiment/internal/ws"
)

// Config controls a benchmark run.
type Config struct {
	WorkDir     string
	CacheDir    string
	Toolbox     string // from SetupToolbox
	Git         string
	CustomBazel string
	StockBazel  string
	BazelDiff   string
	TD          string
	Tools       []string
	Scenarios   []scenario.Scenario
	Scale       int
	PerfReps    int
	Oracle      bool
	KeepServers bool
	Out         io.Writer
	Meta        map[string]string
	// Prior, if set, holds results of an earlier run of the same fixture and
	// scenarios. Its per-scenario ground truth is reused instead of
	// rebuilding with the oracle (only the base revision is built, to know
	// the target universe), and its tools' scores are merged into the output.
	Prior *Results
}

// ToolScore is one tool's result for one scenario, scored against the oracle.
type ToolScore struct {
	Tool          string        `json:"tool"`
	Outcome       tools.Outcome `json:"outcome"`
	TP            int           `json:"true_positives"`
	FN            int           `json:"missed"`
	FP            int           `json:"over_selected"`
	Missed        []string      `json:"missed_targets,omitempty"`
	Extra         []string      `json:"over_selected_targets,omitempty"`
	OutOfUniverse int           `json:"out_of_universe"`
}

// ScenarioResult is the outcome of one scenario.
type ScenarioResult struct {
	Name           string      `json:"name"`
	Category       string      `json:"category"`
	Description    string      `json:"description"`
	Expectation    string      `json:"expectation"`
	Ops            []string    `json:"ops"`
	DiffStat       string      `json:"diffstat"`
	Truth          []string    `json:"truth,omitempty"`
	OutputsChanged []string    `json:"outputs_changed,omitempty"`
	Removed        []string    `json:"removed,omitempty"`
	Universe       int         `json:"universe"`
	OracleSeconds  float64     `json:"oracle_seconds"`
	OracleFailed   []string    `json:"oracle_failed,omitempty"`
	HasOracle      bool        `json:"has_oracle"`
	Tools          []ToolScore `json:"tools"`
	Err            string      `json:"error,omitempty"`
}

// Results is everything a run produces.
type Results struct {
	Meta      map[string]string    `json:"meta"`
	Tools     []string             `json:"tools"`
	Scale     int                  `json:"scale"`
	BaseSize  int                  `json:"base_targets"`
	// RuleTargets is the number of rule targets in //... (workspace size).
	RuleTargets int `json:"rule_targets"`
	Scenarios []ScenarioResult     `json:"scenarios"`
	NoChange  map[string][]float64 `json:"no_change_seconds"`
	Started   time.Time            `json:"started"`
	Finished  time.Time            `json:"finished"`
}

// Run executes the benchmark and returns its results. Results are also
// written to <WorkDir>/results.json after every scenario.
func Run(ctx context.Context, cfg Config) (*Results, error) {
	res := &Results{Meta: cfg.Meta, Tools: cfg.Tools, Scale: cfg.Scale, NoChange: map[string][]float64{}, Started: time.Now()}
	say := func(format string, args ...any) { fmt.Fprintf(cfg.Out, format+"\n", args...) }

	repo := filepath.Join(cfg.WorkDir, "repo")
	state := filepath.Join(cfg.WorkDir, "state")
	for _, d := range []string{state, cfg.CacheDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}
	logf, err := os.Create(filepath.Join(cfg.WorkDir, "commands.log"))
	if err != nil {
		return nil, err
	}
	defer logf.Close()
	home := filepath.Join(cfg.WorkDir, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		return nil, err
	}
	// Every process gets a minimal environment: only the toolbox on PATH. No
	// TMPDIR, so build actions use the hermetic sandbox's own /tmp.
	runner := &ws.Runner{Log: logf, Env: []string{
		"PATH=" + filepath.Join(cfg.Toolbox, "bin"),
		"HOME=" + home,
		"USER=hashbench",
		"LANG=C.UTF-8",
	}}

	// A fresh checkout every run; output bases and caches under state/ and
	// CacheDir survive between runs so repeated runs start warm.
	say("Materializing fixture in %s", repo)
	if err := os.RemoveAll(repo); err != nil {
		return nil, err
	}
	if err := ws.Materialize(fixture.FS, fixture.Root, repo); err != nil {
		return nil, err
	}
	if cfg.Scale > 0 {
		say("Generating %d scale packages", cfg.Scale)
		if err := scale.Generate(repo, cfg.Scale); err != nil {
			return nil, err
		}
	}
	userRC := userBazelrc(cfg)
	if err := os.WriteFile(filepath.Join(repo, "user.bazelrc"), []byte(userRC), 0o644); err != nil {
		return nil, err
	}
	// user.bazelrc is committed with the base revision: target-determinator
	// evaluates the base in a separate worktree, which must see the same flags.
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("/bazel-*\n"), 0o644); err != nil {
		return nil, err
	}
	git := &ws.Git{Bin: cfg.Git, Dir: repo, Runner: runner}
	baseRev, err := git.Init(ctx)
	if err != nil {
		return nil, err
	}

	// Workspace size, from a throwaway server so no tool gets a head start.
	countBase := filepath.Join(state, "count", "output_base")
	if q := runner.Run(ctx, repo, cfg.StockBazel, "--output_base="+countBase, "query", "kind(rule, //...)", "--output=label"); q.Err == nil {
		res.RuleTargets = strings.Count(string(q.Stdout), "\n")
		say("Workspace has %d rule targets", res.RuleTargets)
	} else {
		say("  warning: counting targets failed: %s", firstLine(q.Err.Error()))
	}
	runner.Run(ctx, repo, cfg.StockBazel, "--output_base="+countBase, "shutdown")

	env := tools.Env{
		Runner: runner, Workspace: repo, StateDir: state,
		StockBazel: cfg.StockBazel, CustomBazel: cfg.CustomBazel, BazelDiff: cfg.BazelDiff, TD: cfg.TD,
	}
	var ts []tools.Tool
	for _, name := range cfg.Tools {
		t, err := tools.New(name, env)
		if err != nil {
			return nil, err
		}
		ts = append(ts, t)
	}
	var orc *oracle.Oracle
	defer func() {
		if cfg.KeepServers {
			return
		}
		sctx := context.Background()
		for _, t := range ts {
			t.Shutdown(sctx)
		}
		if orc != nil {
			orc.Shutdown(sctx)
		}
	}()

	for _, t := range ts {
		start := time.Now()
		say("Preparing %s on the base revision (first run fetches and loads everything)...", t.Name())
		if err := t.Prepare(ctx, baseRev); err != nil {
			return nil, fmt.Errorf("prepare %s: %w", t.Name(), err)
		}
		say("  %s ready in %s", t.Name(), time.Since(start).Round(time.Second))
	}

	var base oracle.Snapshot
	if cfg.Oracle {
		orc = &oracle.Oracle{
			Runner: runner, Bazel: cfg.StockBazel, Workspace: repo,
			OutputBase: filepath.Join(state, "oracle", "output_base"),
			RepoDir:    filepath.Join(state, "oracle", "repo"),
		}
		if err := os.MkdirAll(filepath.Dir(orc.OutputBase), 0o755); err != nil {
			return nil, err
		}
		if err := orc.Install(); err != nil {
			return nil, err
		}
		say("Building the base revision with the oracle aspect (builds every target)...")
		base, err = orc.Build(ctx, "base")
		if err != nil {
			return nil, fmt.Errorf("oracle base build: %w", err)
		}
		res.BaseSize = len(base.Inputs)
		say("  oracle: %d targets fingerprinted, %d failed, %s", len(base.Inputs), len(base.Failed), base.Duration.Round(time.Second))
	}

	prior := map[string]ScenarioResult{}
	if cfg.Prior != nil {
		for _, sc := range cfg.Prior.Scenarios {
			prior[sc.Name] = sc
		}
	}

	save := func() {
		b, _ := json.MarshalIndent(res, "", " ")
		os.WriteFile(filepath.Join(cfg.WorkDir, "results.json"), b, 0o644)
	}

	if cfg.Prior != nil {
		for _, t := range cfg.Prior.Tools {
			if !contains(res.Tools, t) {
				res.Tools = append(res.Tools, t)
				res.NoChange[t] = cfg.Prior.NoChange[t]
			}
		}
	}
	for i, sc := range cfg.Scenarios {
		say("[%d/%d] %s: %s", i+1, len(cfg.Scenarios), sc.Name, sc.Description)
		sr := runScenario(ctx, cfg, git, baseRev, sc, ts, orc, base, res, say, prior)
		res.Scenarios = append(res.Scenarios, sr)
		save()
	}
	if err := git.Reset(ctx, baseRev); err != nil {
		return res, err
	}
	res.Finished = time.Now()
	save()
	return res, nil
}

func runScenario(ctx context.Context, cfg Config, git *ws.Git, baseRev string, sc scenario.Scenario,
	ts []tools.Tool, orc *oracle.Oracle, base oracle.Snapshot, res *Results, say func(string, ...any),
	prior map[string]ScenarioResult) ScenarioResult {
	sr := ScenarioResult{Name: sc.Name, Category: sc.Category, Description: sc.Description, Expectation: sc.Expectation}
	for _, op := range sc.Ops {
		sr.Ops = append(sr.Ops, op.String())
	}
	if err := git.Reset(ctx, baseRev); err != nil {
		sr.Err = err.Error()
		return sr
	}
	for _, op := range sc.Ops {
		if err := op.Apply(git.Dir); err != nil {
			sr.Err = fmt.Sprintf("apply %s: %v", op, err)
			say("  ERROR %s", sr.Err)
			return sr
		}
	}
	headRev, err := git.Commit(ctx, sc.Name)
	if err != nil {
		sr.Err = err.Error()
		return sr
	}
	sr.DiffStat = git.DiffStat(ctx, baseRev, headRev)

	// The control scenario doubles as the warm no-change timing sample.
	reps := 1
	if sc.Name == "control" && cfg.PerfReps > 1 {
		reps = cfg.PerfReps
	}
	outcomes := map[string]tools.Outcome{}
	for _, t := range ts {
		for r := 0; r < reps; r++ {
			o := t.Affected(ctx, baseRev, headRev)
			if sc.Name == "control" {
				res.NoChange[t.Name()] = append(res.NoChange[t.Name()], o.Duration.Seconds())
			}
			outcomes[t.Name()] = o
		}
		o := outcomes[t.Name()]
		status := fmt.Sprintf("%d affected", len(o.Affected))
		if o.Err != "" {
			status = "ERROR: " + firstLine(o.Err)
		}
		say("  %-20s %7.1fs  %s", t.Name(), o.Duration.Seconds(), status)
	}

	universe := map[string]bool{}
	truth := map[string]bool{}
	if p, ok := prior[sc.Name]; ok && p.HasOracle && orc != nil {
		// Reuse the earlier run's ground truth for this scenario.
		sr.HasOracle = true
		sr.Truth, sr.Removed, sr.OutputsChanged = p.Truth, p.Removed, p.OutputsChanged
		sr.OracleSeconds, sr.OracleFailed = p.OracleSeconds, p.OracleFailed
		removedSet := map[string]bool{}
		for _, l := range p.Removed {
			removedSet[l] = true
		}
		for l := range base.Inputs {
			if !removedSet[l] {
				universe[l] = true
			}
		}
		for _, l := range p.Truth {
			universe[l] = true
			truth[l] = true
		}
		sr.Universe = len(universe)
		say("  %-20s          %d need rebuild (ground truth from prior run)", "oracle", len(truth))
	} else if orc != nil {
		head, err := orc.Build(ctx, sc.Name)
		sr.OracleSeconds = head.Duration.Seconds()
		if err != nil {
			sr.Err = "oracle: " + err.Error()
			say("  ERROR %s", firstLine(sr.Err))
		} else {
			sr.HasOracle = true
			failed := map[string]bool{}
			for _, l := range append(append([]string{}, base.Failed...), head.Failed...) {
				failed[l] = true
			}
			sr.OracleFailed = sortedKeys(failed)
			outputs := map[string]bool{}
			for l, d := range head.Inputs {
				if failed[l] {
					continue
				}
				universe[l] = true
				if base.Inputs[l] != d {
					truth[l] = true
				}
				if base.Outputs[l] != head.Outputs[l] {
					outputs[l] = true
				}
			}
			sr.OutputsChanged = sortedKeys(outputs)
			for l := range base.Inputs {
				if _, ok := head.Inputs[l]; !ok && !failed[l] {
					sr.Removed = append(sr.Removed, l)
				}
			}
			sort.Strings(sr.Removed)
			sr.Truth = sortedKeys(truth)
			sr.Universe = len(universe)
			say("  %-20s %7.1fs  %d need rebuild (ground truth), %d with changed outputs", "oracle", sr.OracleSeconds, len(truth), len(outputs))
		}
	}

	removed := map[string]bool{}
	for _, l := range sr.Removed {
		removed[l] = true
	}
	for _, t := range ts {
		o := outcomes[t.Name()]
		score := ToolScore{Tool: t.Name(), Outcome: o}
		sort.Strings(score.Outcome.Labels)
		if sr.HasOracle && o.Err == "" {
			for l := range o.Affected {
				switch {
				case removed[l]:
				case !universe[l]:
					score.OutOfUniverse++
				case truth[l]:
					score.TP++
				default:
					score.FP++
					score.Extra = append(score.Extra, l)
				}
			}
			for l := range truth {
				if !o.Affected[l] {
					score.FN++
					score.Missed = append(score.Missed, l)
				}
			}
			sort.Strings(score.Missed)
			sort.Strings(score.Extra)
		}
		sr.Tools = append(sr.Tools, score)
	}
	if p, ok := prior[sc.Name]; ok {
		have := map[string]bool{}
		for _, t := range sr.Tools {
			have[t.Tool] = true
		}
		for _, t := range p.Tools {
			if !have[t.Tool] {
				sr.Tools = append(sr.Tools, t)
			}
		}
	}
	return sr
}

// userBazelrc returns the machine-specific Bazel flags written into the
// benchmark workspace: shared caches, and the toolbox's static bash as the
// shell for every action.
func userBazelrc(cfg Config) string {
	lines := []string{
		// No MODULE.bazel.lock: it would be an untracked, tool-dependent file.
		"common --lockfile_mode=off",
		"common --repository_cache=" + filepath.Join(cfg.CacheDir, "repository"),
		"common --repo_contents_cache=" + filepath.Join(cfg.CacheDir, "repo_contents"),
		"build --disk_cache=" + filepath.Join(cfg.CacheDir, "disk"),
		"build --shell_executable=" + filepath.Join(cfg.Toolbox, "bin", "bash"),
	}
	return strings.Join(lines, "\n") + "\n"
}

// SetupToolbox creates dir/bin holding the static bash and busybox (with a
// relative symlink per busybox applet) plus git. It is the only PATH every
// process the harness starts sees.
func SetupToolbox(dir, bash, busybox, git string) error {
	bin := filepath.Join(dir, "bin")
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return err
	}
	for name, src := range map[string]string{"bash": bash, "busybox": busybox} {
		b, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(bin, name), b, 0o755); err != nil {
			return err
		}
	}
	out, err := exec.Command(filepath.Join(bin, "busybox"), "--list").Output()
	if err != nil {
		return fmt.Errorf("busybox --list: %w", err)
	}
	for _, applet := range strings.Fields(string(out)) {
		if applet == "bash" || applet == "busybox" {
			continue
		}
		if err := os.Symlink("busybox", filepath.Join(bin, applet)); err != nil {
			return err
		}
	}
	// git versions the benchmark workspace; target-determinator runs it too.
	return os.Symlink(git, filepath.Join(bin, "git"))
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(s, "\n")
	return s
}
