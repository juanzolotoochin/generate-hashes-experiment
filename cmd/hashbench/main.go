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
		workDir     = flag.String("work-dir", "hashbench-work", "benchmark checkout, output bases and results (relative to where you ran bazel run)")
		cacheDir    = flag.String("cache-dir", filepath.Join(home, ".cache", "hashbench"), "shared repository, repo-contents and disk caches")
		toolList    = flag.String("tools", "custom,bazel-diff,target-determinator", "comma-separated tools to benchmark")
		only        = flag.String("scenarios", "", "comma-separated scenario names or categories (default: all; for scaling runs a representative set)")
		scaleList   = flag.String("scale", "0", "comma-separated workspace sizes: add N generated service packages (~8 targets each); several values run a scaling sweep")
		perfReps    = flag.Int("perf-reps", 5, "warm repetitions of the no-change (control) scenario")
		useOracle   = flag.Bool("oracle", true, "build every target to compute ground truth (default: on for scale 0, off otherwise)")
		keepServers = flag.Bool("keep-servers", false, "leave Bazel servers running after the run")
		list        = flag.Bool("list", false, "list scenarios and exit")
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
	tools := map[string]*string{"stock-bazel": stockBazel, "bazel-diff": bazelDiff, "target-determinator": td, "bash": bash, "busybox": busybox}
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	var sweep []*bench.Results
	for _, n := range scales {
		dir := userPath(*workDir)
		if len(scales) > 1 {
			dir = filepath.Join(dir, fmt.Sprintf("scale-%d", n))
		}
		oracleOn := *useOracle
		if !explicit["oracle"] {
			oracleOn = n == 0
		}
		cfg := bench.Config{
			WorkDir: dir, CacheDir: cache, Toolbox: toolbox, Git: git,
			CustomBazel: *customBazel, StockBazel: *stockBazel, BazelDiff: *bazelDiff, TD: *td,
			Tools: toolNames, Scenarios: selected, Scale: n, PerfReps: *perfReps,
			Oracle: oracleOn, KeepServers: *keepServers, Out: os.Stdout,
			Meta: map[string]string{
				"stock_bazel":         "9.2.0",
				"bazel_diff":          "v60.0.0",
				"target_determinator": "v0.34.0",
				"custom_bazel":        filepath.Base(*customBazel),
				"host":                runtime.GOOS + "/" + runtime.GOARCH,
				"cpus":                fmt.Sprint(runtime.NumCPU()),
				"date":                time.Now().Format(time.RFC3339),
			},
		}
		if err := os.MkdirAll(cfg.WorkDir, 0o755); err != nil {
			return err
		}
		fmt.Printf("\n=== scale %d (%s) ===\n", n, cfg.WorkDir)
		res, err := bench.Run(ctx, cfg)
		if res != nil {
			paths, rerr := report.Write(cfg.WorkDir, res)
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
		paths, err := report.WriteScaling(userPath(*workDir), sweep)
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
