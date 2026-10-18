// Command hashbench benchmarks Bazel target-selection tools for correctness
// (against a build-graph oracle) and warm-cache performance.
//
// Run it with the custom Bazel under test:
//
//	/path/to/custom-bazel run //cmd/hashbench -- -custom-bazel=/path/to/custom-bazel
//
// `bazel run` supplies every other tool (stock Bazel, bazel-diff,
// target-determinator, a static bash and busybox) from MODULE.bazel.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/juanzolotoochin/generate-hashes-experiment/internal/bench"
	"github.com/juanzolotoochin/generate-hashes-experiment/internal/oracle"
	"github.com/juanzolotoochin/generate-hashes-experiment/internal/report"
	"github.com/juanzolotoochin/generate-hashes-experiment/internal/scenario"
)

func main() {
	var err error
	if filepath.Base(os.Args[0]) == oracle.DigestToolName {
		// Installed into the oracle's aspect repository as its digest tool.
		err = oracle.RunDigest(os.Args[1:])
	} else {
		err = run()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, filepath.Base(os.Args[0])+":", err)
		os.Exit(1)
	}
}

// perfScenarios is the default scenario set for scaling runs: the no-change
// case plus one representative change per kind.
var perfScenarios = []string{
	"control", "go_src_edit", "py_src_edit", "cc_header_edit", "rust_src_edit",
	"data_file_edit", "build_copts", "bzl_macro", "go_dep_bump", "module_comment",
}

func run() error {
	home, _ := os.UserHomeDir()
	var (
		customBazel = flag.String("custom-bazel", "", "Bazel binary that supports `query --generate_hashes` (required when the custom tool is selected)")
		stockBazel  = flag.String("stock-bazel", "", "stock Bazel for bazel-diff, target-determinator and the oracle (set by bazel run)")
		bazelDiff   = flag.String("bazel-diff", "", "bazel-diff binary (set by bazel run)")
		td          = flag.String("target-determinator", "", "target-determinator binary (set by bazel run)")
		bash        = flag.String("bash", "", "static bash (set by bazel run)")
		busybox     = flag.String("busybox", "", "static busybox (set by bazel run)")
		workDir     = flag.String("work-dir", "hashbench-work", "benchmark checkout, output bases and logs (relative to where you ran bazel run)")
		resultsDir  = flag.String("results-dir", "", "where results.json, report.html and summary.md go, rewritten after every scenario (default: results/ in the source tree under bazel run, else <work-dir>)")
		cacheDir    = flag.String("cache-dir", filepath.Join(home, ".cache", "hashbench"), "shared repository, repo-contents and disk caches")
		toolList    = flag.String("tools", "custom,bazel-diff,target-determinator", "comma-separated tools to benchmark")
		only        = flag.String("scenarios", "", "comma-separated scenario names or categories (default: all; for scaling runs a representative set)")
		scaleList   = flag.String("scale", "0", "comma-separated workspace sizes: add N generated service packages (~8 targets each); several values run a scaling sweep")
		perfReps    = flag.Int("perf-reps", 5, "warm repetitions of the no-change (control) scenario")
		useOracle   = flag.Bool("oracle", true, "build every target to compute ground truth (default: on for scale 0, off otherwise)")
		keepServers = flag.Bool("keep-servers", false, "leave Bazel servers running after the run")
		list        = flag.Bool("list", false, "list scenarios and exit")
		extraRC     = flag.String("bazelrc", "", "comma-separated extra bazelrc files imported by every Bazel invocation in the benchmark (e.g. remote cache flags and credentials)")
		cacheMode   = flag.String("repo-cache-mode", "local", "local: tools fetch repositories from the internet into their own caches; remote: also use --experimental_remote_repo_contents_cache with -remote-cache (tools are reported as <tool>@remote)")
		remoteCache = flag.String("remote-cache", "local", "remote cache for -repo-cache-mode=remote: local (the bundled cache server on localhost, data in -cache-dir) or a grpc:// or grpcs:// URL (credentials go in a -bazelrc file)")
		rcServer    = flag.String("remote-cache-server", "", "the bundled local remote cache server (set by bazel run)")
		updLocks    = flag.Bool("update-lockfiles", false, "regenerate fixture/_workspace/MODULE.bazel.lock and the scenarios' lockfiles in the source tree (run with bazel run) and exit")
		rereport    = flag.Bool("rereport", false, "regenerate the report from <results-dir>/results.json (adding the repo contents cache support table if missing) and exit")
		seedOnly    = flag.Bool("seed-only", false, "with -repo-cache-mode=remote: seed the remote repo contents cache with the base revision's repositories and exit")
		reuseTruth  = flag.String("reuse-truth", "", "results.json of an earlier run: reuse its ground truth (skips per-scenario oracle builds) and merge its tools' scores")
	)
	flag.Parse()

	all := scenario.All()
	if *list {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tCATEGORY\tDESCRIPTION")
		for _, s := range all {
			fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, s.Category, s.Description)
		}
		return w.Flush()
	}
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		return fmt.Errorf("only linux/amd64 hosts are supported (the fixture pins linux_x86_64 platforms); got %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	scales, err := parseScales(*scaleList)
	if err != nil {
		return err
	}
	explicit := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	if *only == "" && (len(scales) > 1 || scales[0] > 0) {
		*only = strings.Join(perfScenarios, ",")
	}
	selected, err := selectScenarios(all, *only)
	if err != nil {
		return err
	}
	toolNames := splitList(*toolList)

	// Paths from `bazel run` args are relative to the runfiles directory (the
	// current directory); paths the user typed are relative to where they ran
	// `bazel run`.
	userDir := os.Getenv("BUILD_WORKING_DIRECTORY")
	userPath := func(p string) string {
		if p == "" || filepath.IsAbs(p) {
			return p
		}
		if userDir != "" {
			return filepath.Join(userDir, p)
		}
		a, _ := filepath.Abs(p)
		return a
	}
	results := userPath(*resultsDir)
	if results == "" {
		if src := os.Getenv("BUILD_WORKSPACE_DIRECTORY"); src != "" {
			results = filepath.Join(src, "results")
		} else {
			results = userPath(*workDir)
		}
	}
	if *rereport {
		dir := results
		b, err := os.ReadFile(filepath.Join(dir, "results.json"))
		if err != nil {
			return err
		}
		var res bench.Results
		if err := json.Unmarshal(b, &res); err != nil {
			return err
		}
		if len(res.RepoSupport) == 0 {
			res.RepoSupport = bench.RepoSupportFor(userPath(*workDir))
			b, _ := json.MarshalIndent(&res, "", " ")
			if err := os.WriteFile(filepath.Join(dir, "results.json"), b, 0o644); err != nil {
				return err
			}
		}
		paths, err := report.Write(dir, &res)
		if err != nil {
			return err
		}
		fmt.Println("Report:\n  " + strings.Join(paths, "\n  "))
		return nil
	}
	var prior *bench.Results
	if *reuseTruth != "" {
		b, err := os.ReadFile(userPath(*reuseTruth))
		if err != nil {
			return err
		}
		prior = &bench.Results{}
		if err := json.Unmarshal(b, prior); err != nil {
			return fmt.Errorf("-reuse-truth: %w", err)
		}
	}
	runfile := func(name, p string) (string, error) {
		if p == "" {
			return "", fmt.Errorf("-%s is required (run hashbench with `bazel run //cmd/hashbench`)", name)
		}
		a, err := filepath.Abs(p)
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(a); err != nil {
			return "", fmt.Errorf("-%s: %w", name, err)
		}
		return a, nil
	}
	tools := map[string]*string{"stock-bazel": stockBazel, "bazel-diff": bazelDiff, "target-determinator": td, "bash": bash, "busybox": busybox, "remote-cache-server": rcServer}
	for name, p := range tools {
		if *p, err = runfile(name, *p); err != nil {
			return err
		}
	}
	for _, t := range toolNames {
		if t == "custom" {
			if *customBazel == "" {
				return fmt.Errorf("-custom-bazel is required (or drop custom from -tools)")
			}
			if *customBazel, err = runfile("custom-bazel", userPath(*customBazel)); err != nil {
				return err
			}
		}
	}
	git, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("git is required to version the benchmark workspace: %w", err)
	}

	cache := userPath(*cacheDir)
	toolbox := filepath.Join(cache, "toolbox")
	if err := bench.SetupToolbox(toolbox, *bash, *busybox, git); err != nil {
		return fmt.Errorf("toolbox: %w", err)
	}

	if *cacheMode != "local" && *cacheMode != "remote" {
		return fmt.Errorf("-repo-cache-mode must be local or remote, not %q", *cacheMode)
	}
	if *seedOnly && *cacheMode != "remote" {
		return fmt.Errorf("-seed-only needs -repo-cache-mode=remote")
	}
	var rcs, rcNames []string
	for _, f := range splitList(*extraRC) {
		p := userPath(f)
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("-bazelrc: %w", err)
		}
		rcs = append(rcs, p)
		rcNames = append(rcNames, filepath.Base(p))
	}

	remoteURL := *remoteCache
	if *cacheMode == "remote" && remoteURL == "local" {
		addr, stopServer, err := bench.StartLocalCache(*rcServer, filepath.Join(cache, "remotecache"))
		if err != nil {
			return fmt.Errorf("local remote cache: %w", err)
		}
		defer stopServer()
		remoteURL = "grpc://" + addr
		fmt.Printf("Local remote cache on %s (data in %s)\n", addr, filepath.Join(cache, "remotecache"))
	}

	lockSrc := ""
	if *updLocks {
		if lockSrc = os.Getenv("BUILD_WORKSPACE_DIRECTORY"); lockSrc == "" {
			return fmt.Errorf("-update-lockfiles must be run with bazel run")
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	var sweep []*bench.Results
	for _, n := range scales {
		dir, out := userPath(*workDir), results
		if len(scales) > 1 {
			dir = filepath.Join(dir, fmt.Sprintf("scale-%d", n))
			out = filepath.Join(out, fmt.Sprintf("scale-%d", n))
		}
		oracleOn := *useOracle
		if !explicit["oracle"] {
			oracleOn = n == 0
		}
		cfg := bench.Config{
			WorkDir: dir, ResultsDir: out, CacheDir: cache, Toolbox: toolbox, Git: git, ExtraRC: rcs,
			RepoCacheMode: *cacheMode, RemoteCache: remoteURL, RemoteName: *remoteCache, LockfileSrc: lockSrc, SeedOnly: *seedOnly,
			CustomBazel: *customBazel, StockBazel: *stockBazel, BazelDiff: *bazelDiff, TD: *td,
			Tools: toolNames, Scenarios: selected, Scale: n, PerfReps: *perfReps,
			Oracle: oracleOn || prior != nil, KeepServers: *keepServers, Out: os.Stdout, Prior: prior,
			Meta: map[string]string{
				"stock_bazel":         "9.2.0",
				"bazel_diff":          "v60.0.0",
				"target_determinator": "v0.34.0",
				"custom_bazel":        filepath.Base(*customBazel),
				"extra_bazelrc":       strings.Join(rcNames, ","),
				"repo_cache_mode":     *cacheMode,
				"remote_cache":        remoteCacheMeta(*cacheMode, *remoteCache),
				"host":                runtime.GOOS + "/" + runtime.GOARCH,
				"cpus":                fmt.Sprint(runtime.NumCPU()),
				"date":                time.Now().Format(time.RFC3339),
			},
		}
		for _, d := range []string{cfg.WorkDir, cfg.ResultsDir} {
			if err := os.MkdirAll(d, 0o755); err != nil {
				return err
			}
		}
		fmt.Printf("\n=== scale %d (%s, results in %s) ===\n", n, cfg.WorkDir, cfg.ResultsDir)
		cfg.OnSave = func(r *bench.Results) { report.Write(cfg.ResultsDir, r) }
		res, err := bench.Run(ctx, cfg)
		if res != nil {
			paths, rerr := report.Write(cfg.ResultsDir, res)
			if rerr != nil {
				return rerr
			}
			fmt.Println("\nReport:")
			for _, p := range paths {
				fmt.Println("  " + p)
			}
			sweep = append(sweep, res)
		}
		if err != nil {
			return err
		}
	}
	if len(scales) > 1 {
		paths, err := report.WriteScaling(results, sweep)
		if err != nil {
			return err
		}
		fmt.Println("\nScaling report:")
		for _, p := range paths {
			fmt.Println("  " + p)
		}
	}
	return nil
}

func parseScales(s string) ([]int, error) {
	var out []int
	for _, p := range splitList(s) {
		var n int
		if _, err := fmt.Sscan(p, &n); err != nil || n < 0 {
			return nil, fmt.Errorf("invalid -scale value %q", p)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("-scale needs at least one value")
	}
	return out, nil
}

func selectScenarios(all []scenario.Scenario, only string) ([]scenario.Scenario, error) {
	if only == "" {
		return all, nil
	}
	want := map[string]bool{}
	for _, s := range splitList(only) {
		want[s] = true
	}
	var out []scenario.Scenario
	for _, s := range all {
		if want[s.Name] || want[s.Category] {
			out = append(out, s)
		}
	}
	for _, s := range all {
		delete(want, s.Name)
		delete(want, s.Category)
	}
	if len(want) > 0 {
		var missing []string
		for k := range want {
			missing = append(missing, k)
		}
		return nil, fmt.Errorf("unknown scenarios: %s (see -list)", strings.Join(missing, ", "))
	}
	return out, nil
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// remoteCacheMeta names the remote cache in the report without credentials.
func remoteCacheMeta(mode, remote string) string {
	if mode != "remote" {
		return ""
	}
	if remote == "local" {
		return "local (bundled cache on localhost)"
	}
	return remote
}
