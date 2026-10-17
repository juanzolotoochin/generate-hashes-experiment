package bench

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/juanzolotoochin/generate-hashes-experiment/internal/scenario"
	"github.com/juanzolotoochin/generate-hashes-experiment/internal/ws"
)

// updateLockfiles regenerates the fixture's MODULE.bazel.lock and every
// scenario's lockfile in cfg.LockfileSrc. A lockfile is what Bazel writes
// (default --lockfile_mode=update) after running everything the tools run on
// that revision, so no tool needs to rewrite it during a benchmark: stock
// query and cquery of deps(//...), and the custom Bazel's --generate_hashes
// queries. A scenario gets its own lockfile only if it differs from the base's.
func updateLockfiles(ctx context.Context, cfg Config, git *ws.Git, runner *ws.Runner, sharedRC string, say func(string, ...any)) error {
	state := filepath.Join(cfg.WorkDir, "state")
	type step struct {
		bazel, ob string
		args      []string
	}
	steps := []step{
		{cfg.StockBazel, filepath.Join(state, "lock-stock", "output_base"), []string{"query", "deps(//...)", "--output=label", "--keep_going"}},
		{cfg.StockBazel, filepath.Join(state, "lock-stock", "output_base"), []string{"cquery", "deps(//...)", "--output=label", "--keep_going"}},
	}
	if cfg.CustomBazel != "" {
		ob := filepath.Join(state, "lock-custom", "output_base")
		steps = append(steps,
			step{cfg.CustomBazel, ob, []string{"query", "//...", "--generate_hashes"}},
			step{cfg.CustomBazel, ob, []string{"cquery", "//...", "--generate_hashes"}})
	}
	defer func() {
		for _, st := range steps {
			runner.Run(context.Background(), git.Dir, st.bazel, "--bazelrc="+sharedRC, "--output_base="+st.ob, "shutdown")
		}
	}()
	lockPath := filepath.Join(git.Dir, "MODULE.bazel.lock")
	generate := func(rev string) ([]byte, error) {
		start := time.Now()
		for _, st := range steps {
			r := runner.Run(ctx, git.Dir, st.bazel, append([]string{"--bazelrc=" + sharedRC, "--output_base=" + st.ob}, st.args...)...)
			if r.Err != nil {
				say("  %s: %s %s failed (see commands.log)", rev, filepath.Base(st.bazel), st.args[0])
			}
		}
		b, err := os.ReadFile(lockPath)
		say("  %-28s %6.1fs  %d bytes", rev, time.Since(start).Seconds(), len(b))
		return b, err
	}

	// The base lockfile, from scratch.
	say("Generating the base lockfile...")
	os.Remove(lockPath)
	baseLock, err := generate("base")
	if err != nil {
		return err
	}
	dst := filepath.Join(cfg.LockfileSrc, "fixture", "_workspace", "MODULE.bazel.lock")
	if err := os.WriteFile(dst, baseLock, 0o644); err != nil {
		return err
	}
	baseRev, err := git.Commit(ctx, "base with lockfile")
	if err != nil {
		return err
	}

	say("Generating scenario lockfiles...")
	for _, sc := range cfg.Scenarios {
		if err := git.Reset(ctx, baseRev); err != nil {
			return err
		}
		for _, op := range sc.Ops {
			if err := op.Apply(git.Dir); err != nil {
				return err
			}
		}
		lock, err := generate(sc.Name)
		if err != nil {
			return err
		}
		src := filepath.Join(cfg.LockfileSrc, scenario.LockfileSource(sc.Name))
		if bytes.Equal(lock, baseLock) {
			os.Remove(src)
			continue
		}
		if err := os.WriteFile(src, lock, 0o644); err != nil {
			return err
		}
		say("  %-28s wrote %s", "", scenario.LockfileSource(sc.Name))
	}
	return git.Reset(ctx, baseRev)
}
