// Package bench runs the scenarios through every tool and the oracle.
package bench

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/juanzolotoochin/generate-hashes-experiment/fixture"
	"github.com/juanzolotoochin/generate-hashes-experiment/internal/netmeter"
	"github.com/juanzolotoochin/generate-hashes-experiment/internal/oracle"
	"github.com/juanzolotoochin/generate-hashes-experiment/internal/scale"
	"github.com/juanzolotoochin/generate-hashes-experiment/internal/scenario"
	"github.com/juanzolotoochin/generate-hashes-experiment/internal/tools"
	"github.com/juanzolotoochin/generate-hashes-experiment/internal/ws"
)

// Config controls a benchmark run.
type Config struct {
	WorkDir    string // checkout, output bases and logs
	ResultsDir string // results.json (and the report, via OnSave); defaults to WorkDir
	CacheDir string
	Toolbox  string // from SetupToolbox
	Git      string
	ExtraRC  []string // optional bazelrc files imported by every generated bazelrc
	// RepoCacheMode is "local" (repositories are fetched from the internet
	// into each tool's own caches) or "remote" (also with
	// --experimental_remote_repo_contents_cache against RemoteCache).
	RepoCacheMode string
	RemoteCache   string // grpc:// or grpcs:// URL
	RemoteName    string // "local" or the URL, keys the seeder's state
	SeedOnly      bool   // remote mode: seed the base revision and stop
	// Determinism, if set, carries the determinism results of an earlier
	// -determinism run into this run's results.
	Determinism     []DetResult
	DeterminismMeta map[string]string
	// OnDetSave, in a determinism run, is called after every tool.
	OnDetSave func([]DetResult)
	// LockfileSrc, if set, is the source checkout: regenerate the fixture's
	// MODULE.bazel.lock and the scenarios' lockfiles there, and stop.
	LockfileSrc string
	// OnSave, if set, is called after every scenario with the results so far
	// (e.g. to rewrite the report incrementally).
	OnSave      func(*Results)
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
	Meta     map[string]string `json:"meta"`
	Tools    []string          `json:"tools"`
	Scale    int               `json:"scale"`
	BaseSize int               `json:"base_targets"`
	// RuleTargets is the number of rule targets in //... (workspace size).
	RuleTargets int                  `json:"rule_targets"`
	Scenarios   []ScenarioResult     `json:"scenarios"`
	NoChange    map[string][]float64 `json:"no_change_seconds"`
	// Setup is each tool's first run on the base revision, starting from its
	// own empty caches.
	Setup map[string]Setup `json:"setup"`
	// RepoSupport is the repo contents cache support of every repository the
	// tools fetched for the base revision, by module.
	RepoSupport []RepoSupport `json:"repo_support,omitempty"`
	// BaseHashes are each hash-based tool's per-target hashes of the base
	// revision, to compare the same revision across modes (consistency).
	BaseHashes map[string]map[string]string `json:"base_hashes,omitempty"`
	// Determinism is the latest -determinism run (see determinism.go), with
	// its own metadata: it is run separately and carried across runs.
	Determinism     []DetResult       `json:"determinism,omitempty"`
	DeterminismMeta map[string]string `json:"determinism_meta,omitempty"`
	Started    time.Time                    `json:"started"`
	Finished   time.Time                    `json:"finished"`
}

// Setup is a tool's preparation on the base revision.
type Setup struct {
	Seconds  float64         `json:"seconds"`
	Download netmeter.Counts `json:"download_bytes"`
}

// Run executes the benchmark and returns its results. Results are also
// written to <ResultsDir>/results.json after every scenario.
func Run(ctx context.Context, cfg Config) (*Results, error) {
	res := &Results{Meta: cfg.Meta, Scale: cfg.Scale, NoChange: map[string][]float64{}, Setup: map[string]Setup{}, BaseHashes: map[string]map[string]string{}, Started: time.Now(),
		Determinism: cfg.Determinism, DeterminismMeta: cfg.DeterminismMeta}
	remote := cfg.RepoCacheMode == "remote"
	suffix := ""
	if remote {
		suffix = "@remote"
	}
	for _, t := range cfg.Tools {
		res.Tools = append(res.Tools, t+suffix)
	}
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

	// The oracle (and the target count) share the caches in CacheDir: they
	// are not measured. Every tool gets its own, see toolBazelrc.
	sharedRC := filepath.Join(state, "shared.bazelrc")
	if err := os.WriteFile(sharedRC, []byte(sharedBazelrc(cfg)), 0o644); err != nil {
		return nil, err
	}

	// Workspace size, from a throwaway server so no tool gets a head start.
	countBase := filepath.Join(state, "count", "output_base")
	if q := runner.Run(ctx, repo, cfg.StockBazel, "--bazelrc="+sharedRC, "--output_base="+countBase, "query", "kind(rule, //...)", "--output=label"); q.Err == nil {
		res.RuleTargets = strings.Count(string(q.Stdout), "\n")
		say("Workspace has %d rule targets", res.RuleTargets)
	} else {
		say("  warning: counting targets failed: %s", firstLine(q.Err.Error()))
	}
	runner.Run(ctx, repo, cfg.StockBazel, "--bazelrc="+sharedRC, "--output_base="+countBase, "shutdown")
	if cfg.LockfileSrc != "" {
		return nil, updateLockfiles(ctx, cfg, git, runner, sharedRC, say)
	}

	// Each tool's network traffic goes through its own meter.
	remoteAddr := ""
	if remote {
		if remoteAddr, err = hostPort(cfg.RemoteCache); err != nil {
			return nil, err
		}
	}
	sockDir, err := os.MkdirTemp("", "hashbench-net")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(sockDir)
	meters := map[string]*netmeter.Meter{}
	var ts []tools.Tool
	for i, name := range cfg.Tools {
		dir := filepath.Join(state, name)
		if err := resetForMode(dir, cfg.RepoCacheMode); err != nil {
			return nil, err
		}
		m, err := netmeter.Start(remoteAddr, filepath.Join(sockDir, fmt.Sprintf("%d.sock", i)))
		if err != nil {
			return nil, err
		}
		defer m.Close()
		rc := filepath.Join(dir, "tool.bazelrc")
		if err := os.WriteFile(rc, []byte(toolBazelrc(cfg, dir, m.RemoteProxy())), 0o644); err != nil {
			return nil, err
		}
		env := tools.Env{
			Runner:    &ws.Runner{Log: logf, Env: append(append([]string{}, runner.Env...), m.Env()...)},
			Workspace: repo, StateDir: state, RC: rc, Suffix: suffix,
			StockBazel: cfg.StockBazel, CustomBazel: cfg.CustomBazel, BazelDiff: cfg.BazelDiff, TD: cfg.TD,
		}
		t, err := tools.New(name, env)
		if err != nil {
			return nil, err
		}
		meters[t.Name()] = m
		ts = append(ts, t)
	}

	// In remote mode a separate seeder fetches each revision's repositories
	// first, so they are in the remote repo contents cache before any tool
	// asks for them. It runs what the tools run (the custom Bazel's
	// --generate_hashes queries, stock query and cquery), each binary in its
	// own output base. Its state lives in CacheDir so it stays warm across work
	// dirs; put already-downloaded archives in CacheDir/seed/cache/repository
	// to seed without downloading them again.
	seed := func(string) {}
	if remote {
		// One seeder per remote cache: a seeder that already has every
		// repository locally would not upload anything to a new cache.
		// Downloaded archives are shared (CacheDir/seed/cache/repository).
		seedDir := filepath.Join(cfg.CacheDir, "seed-"+sanitize(cfg.RemoteName))
		if err := os.MkdirAll(seedDir, 0o755); err != nil {
			return nil, err
		}
		sm, err := netmeter.Start(remoteAddr, filepath.Join(sockDir, "seed.sock"))
		if err != nil {
			return nil, err
		}
		defer sm.Close()
		seedRC := filepath.Join(seedDir, "seed.bazelrc")
		rc := toolBazelrc(cfg, seedDir, sm.RemoteProxy())
		rc = strings.Replace(rc, filepath.Join(seedDir, "cache", "repository"), filepath.Join(cfg.CacheDir, "seed", "cache", "repository"), 1)
		if err := os.WriteFile(seedRC, []byte(rc), 0o644); err != nil {
			return nil, err
		}
		seedRunner := &ws.Runner{Log: logf, Env: append(append([]string{}, runner.Env...), sm.Env()...)}
		type step struct {
			bazel, ob string
			args      []string
		}
		var steps []step
		if cfg.CustomBazel != "" {
			ob := filepath.Join(seedDir, "custom", "output_base")
			steps = append(steps,
				step{cfg.CustomBazel, ob, []string{"query", "//...", tools.HashFlag}},
				step{cfg.CustomBazel, ob, []string{"cquery", "//...", tools.HashFlag}})
		}
		ob := filepath.Join(seedDir, "stock", "output_base")
		steps = append(steps,
			step{cfg.StockBazel, ob, []string{"query", "deps(//...)", "--output=label"}},
			step{cfg.StockBazel, ob, []string{"cquery", "deps(//...)", "--output=label"}})
		seed = func(rev string) {
			for _, st := range steps {
				start, c0 := time.Now(), sm.Counts()
				args := append([]string{"--bazelrc=" + seedRC, "--output_base=" + st.ob}, st.args...)
				r := seedRunner.Run(ctx, repo, st.bazel, append(args, "--keep_going")...)
				d := sm.Counts().Sub(c0)
				status := ""
				if r.Err != nil {
					status = " (failed, see commands.log)"
				}
				say("  %-20s %7.1fs  %s %s: uploaded %.1f MB, downloaded %s%s", "seed", time.Since(start).Seconds(),
					filepath.Base(st.bazel), st.args[0], float64(d.Sent)/1e6, MB(d), status)
			}
			say("  %-20s seeded %s", "seed", rev)
		}
		defer func() {
			for _, ob := range []string{filepath.Join(seedDir, "custom", "output_base"), filepath.Join(seedDir, "stock", "output_base")} {
				b := cfg.StockBazel
				if strings.Contains(ob, "custom") {
					b = cfg.CustomBazel
				}
				if b != "" {
					seedRunner.Run(context.Background(), repo, b, "--bazelrc="+seedRC, "--output_base="+ob, "shutdown")
				}
			}
		}()
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

	seed("base")
	if cfg.SeedOnly {
		return nil, nil
	}
	for _, t := range ts {
		start := time.Now()
		say("Preparing %s on the base revision (first run fetches and loads everything)...", t.Name())
		c0 := meters[t.Name()].Counts()
		if err := t.Prepare(ctx, baseRev); err != nil {
			return nil, fmt.Errorf("prepare %s: %w", t.Name(), err)
		}
		d := meters[t.Name()].Counts().Sub(c0)
		res.Setup[t.Name()] = Setup{Seconds: time.Since(start).Seconds(), Download: d}
		if h := t.BaseHashes(); h != nil {
			res.BaseHashes[t.Name()] = h
		}
		say("  %s ready in %s, downloaded %s", t.Name(), time.Since(start).Round(time.Second), MB(d))
	}
	if dirty := git.Dirty(ctx); len(dirty) > 0 {
		say("  warning: preparing the tools modified the checkout: %s (restored)", strings.Join(dirty, ", "))
		git.Reset(ctx, baseRev)
	}
	// Only meaningful in local mode: there a cacheable repo is a symlink into
	// the local repo contents cache. In remote mode it is a lazy directory, so
	// a remote pass keeps the table from the local pass it is merged with.
	if !remote {
		res.RepoSupport = RepoSupportFor(cfg.WorkDir)
	}

	var base oracle.Snapshot
	if cfg.Oracle {
		orc = &oracle.Oracle{
			Runner: runner, Bazel: cfg.StockBazel, Workspace: repo,
			OutputBase: filepath.Join(state, "oracle", "output_base"),
			RepoDir:    filepath.Join(state, "oracle", "repo"),
			RC:         sharedRC,
		}
		if err := os.MkdirAll(filepath.Dir(orc.OutputBase), 0o755); err != nil {
			return nil, err
		}
		if err := orc.Install(); err != nil {
			return nil, err
		}
		say("Building the base revision with the oracle aspect (builds every target)...")
		base, err = orc.Build(ctx, "base")
		if dirty := git.Dirty(ctx); len(dirty) > 0 {
			say("  warning: the oracle modified the checkout: %s (restored)", strings.Join(dirty, ", "))
			git.Reset(ctx, baseRev)
		}
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
		dir := cfg.ResultsDir
		if dir == "" {
			dir = cfg.WorkDir
		}
		os.WriteFile(filepath.Join(dir, "results.json"), b, 0o644)
		if cfg.OnSave != nil {
			cfg.OnSave(res)
		}
	}

	if cfg.Prior != nil {
		// The prior run's tools come first (e.g. local, then @remote).
		var priorTools []string
		for _, t := range cfg.Prior.Tools {
			if !contains(res.Tools, t) {
				priorTools = append(priorTools, t)
				res.NoChange[t] = cfg.Prior.NoChange[t]
				if s, ok := cfg.Prior.Setup[t]; ok {
					res.Setup[t] = s
				}
				if len(res.RepoSupport) == 0 {
					res.RepoSupport = cfg.Prior.RepoSupport
				}
				if h, ok := cfg.Prior.BaseHashes[t]; ok {
					res.BaseHashes[t] = h
				}
			}
		}
		res.Tools = append(priorTools, res.Tools...)
	}
	for i, sc := range cfg.Scenarios {
		say("[%d/%d] %s: %s", i+1, len(cfg.Scenarios), sc.Name, sc.Description)
		sr := runScenario(ctx, cfg, git, baseRev, sc, ts, meters, seed, orc, base, res, say, prior)
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
	ts []tools.Tool, meters map[string]*netmeter.Meter, seed func(string), orc *oracle.Oracle, base oracle.Snapshot, res *Results, say func(string, ...any),
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
	if lock, ok := scenario.Lockfile(sc.Name); ok {
		if err := os.WriteFile(filepath.Join(git.Dir, "MODULE.bazel.lock"), lock, 0o644); err != nil {
			sr.Err = err.Error()
			return sr
		}
	}
	headRev, err := git.Commit(ctx, sc.Name)
	if err != nil {
		sr.Err = err.Error()
		return sr
	}
	sr.DiffStat = git.DiffStat(ctx, baseRev, headRev)
	seed(sc.Name)

	// The control scenario doubles as the warm no-change timing sample.
	reps := 1
	if sc.Name == "control" && cfg.PerfReps > 1 {
		reps = cfg.PerfReps
	}
	outcomes := map[string]tools.Outcome{}
	for _, t := range ts {
		for r := 0; r < reps; r++ {
			c0 := meters[t.Name()].Counts()
			o := t.Affected(ctx, baseRev, headRev)
			d := meters[t.Name()].Counts().Sub(c0)
			o.Download = &d
			if dirty := git.Dirty(ctx); len(dirty) > 0 {
				o.Dirtied = dirty
				say("  %-20s modified the checkout: %s (restored)", t.Name(), strings.Join(dirty, ", "))
				git.Reset(ctx, headRev)
			}
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
		say("  %-20s %7.1fs  %s, downloaded %s", t.Name(), o.Duration.Seconds(), status, MB(*o.Download))
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
		if dirty := git.Dirty(ctx); len(dirty) > 0 {
			say("  warning: the oracle modified the checkout: %s (restored)", strings.Join(dirty, ", "))
			git.Reset(ctx, headRev)
		}
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
	order := map[string]int{}
	for i, t := range res.Tools {
		order[t] = i
	}
	sort.SliceStable(sr.Tools, func(i, j int) bool { return order[sr.Tools[i].Tool] < order[sr.Tools[j].Tool] })
	return sr
}

// userBazelrc returns the machine-specific Bazel flags written into the
// benchmark workspace: the toolbox's static bash as the shell for every
// action. Caches are per process (toolBazelrc, sharedBazelrc).
func userBazelrc(cfg Config) string {
	lines := []string{
		"build --shell_executable=" + filepath.Join(cfg.Toolbox, "bin", "bash"),
	}
	return strings.Join(lines, "\n") + "\n"
}

// sharedBazelrc is passed with --bazelrc to the oracle and the target count:
// caches shared across runs, never measured.
func sharedBazelrc(cfg Config) string {
	lines := []string{
		"common --repository_cache=" + filepath.Join(cfg.CacheDir, "repository"),
		"common --repo_contents_cache=" + filepath.Join(cfg.CacheDir, "repo_contents"),
		"build --disk_cache=" + filepath.Join(cfg.CacheDir, "disk"),
	}
	return strings.Join(append(lines, extraImports(cfg)...), "\n") + "\n"
}

// toolBazelrc is passed with --bazelrc to one tool's Bazel invocations: its
// own repository caches under dir, so its downloads are its own, and in
// remote mode the remote repo contents cache, reached through remoteProxy.
func toolBazelrc(cfg Config, dir, remoteProxy string) string {
	lines := []string{
		"common --repository_cache=" + filepath.Join(dir, "cache", "repository"),
		"common --repo_contents_cache=" + filepath.Join(dir, "cache", "repo_contents"),
	}
	if cfg.RepoCacheMode == "remote" {
		lines = append(lines,
			"startup --experimental_remote_repo_contents_cache",
			"common --remote_cache="+cfg.RemoteCache,
		)
		if remoteProxy != "" {
			lines = append(lines, "common --remote_proxy="+remoteProxy)
		}
	}
	return strings.Join(append(lines, extraImports(cfg)...), "\n") + "\n"
}

// extraImports imports the -bazelrc files by path, so their contents (e.g.
// credentials) never enter the benchmark checkout.
func extraImports(cfg Config) []string {
	var lines []string
	for _, rc := range cfg.ExtraRC {
		lines = append(lines, "import "+rc)
	}
	return lines
}

// hostPort turns grpc[s]://host[:port] into host:port.
func hostPort(url string) (string, error) {
	rest, ok := strings.CutPrefix(url, "grpcs://")
	if !ok {
		if rest, ok = strings.CutPrefix(url, "grpc://"); !ok {
			return "", fmt.Errorf("remote cache %q: want a grpc:// or grpcs:// URL", url)
		}
	}
	rest, _, _ = strings.Cut(rest, "/")
	if !strings.Contains(rest, ":") {
		rest += ":443"
	}
	return rest, nil
}

// MB formats downloaded bytes.
func MB(c netmeter.Counts) string {
	s := fmt.Sprintf("%.1f MB", float64(c.Total())/1e6)
	if c.Remote > 0 {
		s += fmt.Sprintf(" (%.1f MB remote)", float64(c.Remote)/1e6)
	}
	return s
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

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '-' {
			return r
		}
		return '_'
	}, strings.ToLower(s))
}

// resetForMode empties a tool's state dir (output bases and caches) when it
// was last used in another repo cache mode, so every mode starts cold from
// the same paths, and records the mode.
func resetForMode(dir, mode string) error {
	marker := filepath.Join(dir, ".mode")
	if b, err := os.ReadFile(marker); err != nil || string(b) != mode {
		if err := removeAll(dir); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(marker, []byte(mode), 0o644)
}

// removeAll removes dir, first making the read-only directories Bazel leaves
// behind writable.
func removeAll(dir string) error {
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			os.Chmod(p, 0o755)
		}
		return nil
	})
	return os.RemoveAll(dir)
}
