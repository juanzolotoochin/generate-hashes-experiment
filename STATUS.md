# Status

Work-in-progress notes: where the benchmark stands and what to do next.

## Done

- **Harness** (`cmd/hashbench`): one Bazel-built Go binary, launched with the
  custom Bazel under test:
  `/path/to/custom-bazel run //cmd/hashbench -- -custom-bazel=/path/to/custom-bazel`.
  Stock Bazel 9.2.0, bazel-diff v60.0.0, target-determinator v0.34.0, a static
  bash and a static busybox are pinned by sha256 in `MODULE.bazel`.
- **Fixture** (`fixture/_workspace`): builds with stock Bazel 9.2.0. It uses
  rules_cc with the hermetic `llvm` toolchain, rules_go/gazelle, rules_python,
  rules_rs, rules_oci, rules_img, rules_apko, rules_distroless and
  rules_multitool, plus a custom rule, a macro and a Go tool (no shell
  actions).
- **42 scenarios** (`-list`): source, BUILD, .bzl, dependency, bzlmod,
  toolchain and config changes. Dependency scenarios swap in real alternate
  lock files from `internal/scenario/data`.
- **Oracle** (`internal/oracle`): builds `//...` with an injected aspect. A
  target "needs rebuild" when the Merkle fingerprint of its action inputs
  changes; this is computed from `aquery` plus source contents, and includes
  the executable bit of source files the target ships. "Outputs changed" is
  reported alongside. Bugs fixed along the way: provenance-dependent modes of
  generated files, files inside tree artifacts, Python venv symlinks, and
  target-determinator's short-form labels.
- **target-determinator** runs in place, without a worktree, because the
  benchmark checkout is always clean (`user.bazelrc` is committed into the
  benchmark repo and `--lockfile_mode=off`).
- **Scaling sweep** (`-scale=0,2000,...`) and `scaling.html` are implemented
  but have not been run yet.

## Known issue: oracle results are not trustworthy yet

The tools (custom, bazel-diff, target-determinator) only run queries, so
their answers and timings are unaffected. The **oracle** really builds every
target, and that build was broken:

- Oracle builds run in Bazel's hermetic sandbox, where `/bin` and `/usr/bin`
  should be the toolbox's static bash and busybox. Both were mounted from the
  same source directory, and Bazel keeps only one mount per source path, so
  `/bin/bash` was missing.
- Every uncached shell action in the oracle build failed. Most were hidden
  because the shared disk cache already held results from earlier,
  non-hermetic runs.
- Targets that failed to build were excluded from scoring, so some reference
  answers were incomplete. The visible ones: `//apko:image` (every scenario),
  the Rust targets (crate bump, Rust toolchain, LLVM toolchain and `.bazelrc`
  scenarios), and some `*.digest` and distroless targets.

**Fix (pushed, not yet verified):** `/bin` and `/usr/bin` now come from
separate directories (`toolbox/bin` and `toolbox/usr-bin`). See
`SetupToolbox` and `userBazelrc` in `internal/bench/bench.go`.

## Preliminary findings (from the affected run, to be confirmed)

- target-determinator: no missed targets in any scenario; over-selects;
  about 10s per change.
- custom and bazel-diff: 0.1–0.3s per change; both miss Go SDK, LLVM
  toolchain and `.bazelrc` (`--copt`) changes.
- custom also misses a runfile's exec-bit change and a rules_multitool tool
  bump.
- bazel-diff over-selects on rules_distroless lock changes (including a
  snapshot move where every package is identical), a `bazel_dep` bump, and
  source-file exec-bit changes.

## Next steps

1. **Verify the sandbox fix.** Run the full benchmark with an empty disk cache
   and repo-contents cache, so the oracle build actually executes every
   action. The download cache can be reused:

   ```sh
   mkdir -p ~/.cache/hashbench-clean
   ln -s ~/.cache/hashbench/repository ~/.cache/hashbench-clean/repository
   /path/to/custom-bazel run //cmd/hashbench -- -custom-bazel=/path/to/custom-bazel \
       -cache-dir=$HOME/.cache/hashbench-clean
   ```

   Check that `oracle_failed` is empty for every scenario in `results.json`.
   If some actions still need host tools, either fix them or drop
   `--experimental_use_hermetic_linux_sandbox` for the oracle (the tools
   don't depend on it).
2. **Publish results.** Add `results/` with `summary.md`, `report.html` and
   `results.json` from that run, after checking them for local paths. Update
   the README with the findings.
3. **Scaling sweep:**
   `-scale=0,2000,8000,16000 -tools=custom,bazel-diff`, then publish
   `scaling.md` and `scaling.html`.
4. **Custom Bazel:** link the fork and its build instructions in the README
   (there's a TODO).
5. Optional:
   - Remove the `HASHBENCH_ORACLE_DEBUG` fingerprint dump in
     `internal/oracle/oracle.go`, or document it.
   - Consider running target-determinator in the scaling sweep at smaller
     sizes.
