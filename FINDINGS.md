# Findings

An interpretation of the results in [`results/`](results/). The tables there are
generated from `results/results.json`; this file is written by hand from them.

The tools:

| Name in the report | What runs |
|---|---|
| generate_hashes (query) | the custom Bazel's `query //... --experimental_generate_hashes` |
| generate_hashes (cquery) | the custom Bazel's `cquery //... --experimental_generate_hashes` |
| bazel-diff | bazel-diff v60.0.0 `generate-hashes`, default options |
| bazel-diff-fine | bazel-diff with `--fineGrainedHashExternalReposFile` (pip, `go_deps` and crate repos) |
| bazel-diff-cquery | bazel-diff with `--useCquery` |
| target-determinator | target-determinator v0.34.0 |

## Scope of the evidence

- **One fixture.** The fixture is a polyglot workspace of 43 targets: C++ with a
  hermetic LLVM toolchain, Go, Python, Rust, OCI images, apko and distroless
  images, and a custom rule. Nothing here measures how the tools behave on a
  workspace with thousands of targets, so the timings are the timings of a
  small workspace.
- **47 scenarios.** Each one is a single change on top of one base revision.
  Ground truth comes from building every target and comparing output digests,
  so "affected" means "an output changed or the target is new", not "an input
  changed".
- **One machine.** Linux x86_64, 16 CPUs, warm Bazel servers for the per-change
  timings.
- **The remote repo contents cache** was a bundled localhost cache, not a
  production one.

## Correctness

| Tool | Scenarios with misses | Missed targets | Over-selected targets | Recall | Precision |
|---|---|---|---|---|---|
| generate_hashes (query) | 1 | 10 | 150 | 94.4% | 52.7% |
| generate_hashes (cquery) | 0 | 0 | 143 | 100.0% | 55.3% |
| bazel-diff | 5 | 60 | 90 | 66.1% | 56.5% |
| bazel-diff-fine | 5 | 57 | 93 | 67.8% | 56.3% |
| bazel-diff-cquery | 2 | 20 | 144 | 88.7% | 52.2% |
| target-determinator | 0 | 0 | 106 | 100.0% | 62.5% |

**Misses.** Only two tools never missed an affected target: generate_hashes
(cquery) and target-determinator. The misses of the others fall into three
groups:

- **Toolchains.** bazel-diff and bazel-diff-fine miss every toolchain change:
  the Go SDK, Rust and LLVM version bumps, and an edit to the Go SDK's patch
  file. Together those are 4 of their 5 scenarios with misses, and 50 of
  bazel-diff's 60 missed targets (47 of 57 for bazel-diff-fine). Toolchains reach targets through toolchain resolution, not
  through dependency edges in the unconfigured graph that bazel-diff hashes.
  bazel-diff-cquery catches the version bumps but not the patch edit (see
  below).
- **Configuration.** A `--copt` added to `.bazelrc` changes the outputs of 10
  C++ targets. Every tool that hashes the unconfigured graph misses it:
  generate_hashes (query), bazel-diff and bazel-diff-fine. This is the only
  miss of generate_hashes (query), and no option of an unconfigured query can
  fix it, because the unconfigured graph doesn't depend on build flags.
- **A tool binary version.** Bumping jq in the rules_multitool lockfile changes
  the outputs of 2 targets. bazel-diff-cquery misses it; default bazel-diff and
  every other tool catch it.

**Over-selection.** All tools over-select, and most of it is shared:

- In 14 scenarios every tool selects the same extra targets, target-determinator
  included (bazel-diff also lists source files, which aren't scored). Examples: a `chmod` of a source file (+11), a visibility change
  (+11), a comment in a `.bzl` file (+3). These targets' inputs changed but
  their outputs didn't. A tool that doesn't build can't tell the difference.
- **generate_hashes over-selects when a referenced file changes but no output
  does.** Rewriting the Go SDK patch to touch only `CONTRIBUTING.md` selects 20
  targets with both generate_hashes modes, and none with the other tools. A
  `rules_oci` patch that only touches a README selects 12 with both
  generate_hashes modes, bazel-diff and bazel-diff-fine, and none with
  bazel-diff-cquery or target-determinator. generate_hashes hashes an external repo as a whole, so any change to
  its files or definition selects every target that depends on it.
- **A bzlmod dependency bump** (`bazel_skylib`) that changes no outputs selects
  37 targets with generate_hashes (query), 5 with generate_hashes (cquery),
  13 with bazel-diff, 16 with bazel-diff-fine, 43 with bazel-diff-cquery and
  none with target-determinator.
- bazel-diff and bazel-diff-fine over-select in two distroless scenarios where
  the other tools are exact or nearly exact
  (`distroless_timestamp` +8, `distroless_pkg_bump` +5 versus +1).

target-determinator has the best precision (62.5%) and the most exact scenarios
(28 of 47). Of the two tools with no misses, generate_hashes (cquery) selects 37
more extra targets over the 47 scenarios (143 versus 106). Most of the
difference is two scenarios where an external repo's files changed without
changing any output (`toolchain_patch_noop` +20, `bzlmod_patch_noop` +12).

## Speed

| Tool | Warm, no change | Warm, median per change | First run on the base revision |
|---|---|---|---|
| generate_hashes (query) | 0.1s | 0.2s | 50s, 413 MB |
| generate_hashes (cquery) | 0.7s | 1.3s | 69s, 820 MB |
| bazel-diff | 0.5s | 0.7s | 19s, 141 MB |
| bazel-diff-fine | 0.6s | 0.8s | 32s, 236 MB |
| bazel-diff-cquery | 4.1s | 5.0s | 68s, 829 MB |
| target-determinator | 9.9s | 11.4s | 80s, 820 MB |

- generate_hashes (query) is the fastest tool per change. generate_hashes
  (cquery) is about 9 times faster per change than target-determinator, with
  the same recall.
- target-determinator's time per change includes evaluating the base revision
  again: it checks out the base commit and analyzes it in every run. The
  hash-based tools compute the base once and reuse the snapshot, so their time
  per change is the cost of hashing one revision.
- bazel-diff-cquery takes about 4 times as long per change as generate_hashes
  (cquery) and about half as long as target-determinator. Its time is almost
  all `generate-hashes`; computing the impacted targets takes 0.2s.
- The first-run downloads follow what each tool loads: tools that analyze
  (cquery-based tools and target-determinator) fetch the toolchains, about 820
  MB here. The unconfigured-query tools fetch less.

## bazel-diff with `--useCquery`

On the scenarios where default bazel-diff misses changes:

| Scenario | Truth | bazel-diff | bazel-diff-cquery |
|---|---|---|---|
| go_toolchain | 18 | 18 missed | +2 extra |
| rust_toolchain | 3 | 3 missed | exact |
| llvm_toolchain | 11 | 11 missed | +18 extra |
| bazelrc_copt | 10 | 10 missed | +30 extra |
| toolchain_patch_edit | 18 | 18 missed | 18 missed |
| multitool_bump | 2 | exact | 2 missed |

Hashing the configured graph closes the toolchain-version and configuration
gaps. It does not catch an edit to a toolchain's patch file: bazel-diff hashes a
toolchain repo by its definition, not by the contents of the files the
definition points to. generate_hashes (cquery) catches that edit. It also
misses a tool version bump that default bazel-diff catches (`multitool_bump`),
and over the 47 scenarios it over-selects more than default bazel-diff (144
versus 90), as much as generate_hashes (cquery).

## Determinism

The determinism suite hashes the same revision twice with one thing changed in
between. A correct tool gives identical hashes, and every target whose hash
differs is a target CI would select with nothing changed.

| Check | generate_hashes (query) | generate_hashes (cquery) | bazel-diff | bazel-diff-fine | bazel-diff-cquery | target-determinator |
|---|---|---|---|---|---|---|
| Output base at another path | 1 of 46 | 1 of 46 | 0 | 0 | 2 of 109 | 0 |
| Remote repo contents cache on, then off | 0 | 0 | 0 | 8 of 109 | 0 | 0 |
| The flag toggled in the same output base | 0 | 0 | 0 | 8 of 109 | 0 | 0 |
| A build between snapshots, remote repo contents cache on | 0 | 0 | 0 | 5 of 109 | 0 | 0 |
| Remote snapshot, then a local build and a local snapshot | 0 | 0 | 0 | 8 of 109 | 0 | 0 |
| Repeat, server restart, another user and home, fresh output base, another checkout path, fresh caches, a local build between | 0 | 0 | 0 | 0 | 0 | 0 |

- **bazel-diff-fine depends on what is on disk.** It reads the files of the
  pip, `go_deps` and crate repos to hash Python and Rust targets. With the
  remote repo contents cache those repos are served lazily and their files
  aren't on disk until something reads them, so the hashes change with the
  flag, with a toggle, and with a build that happens to materialize the files.
  In CI, where the base and head snapshots come from different machines or
  different points in time, this selects targets that didn't change. Default
  bazel-diff doesn't read external repo files and isn't affected.
- **generate_hashes depends on the output base path in one case:** a fetched
  repo that writes its own absolute path into one of its files
  (`//hazard:path_content` in the suite). The repo is hashed by its file
  contents and the path is part of them. Two other shapes are handled: an
  absolute symlink in a fetched repo, and a local repo with its path in a rule
  attribute (the shape of rules_java's `local_jdk`, which made every test
  target differ across output base paths with an earlier build).
- **bazel-diff-cquery depends on the output base path for a local repo** whose
  path is in a rule attribute (`//hazard:path_attr` and its file). That is the
  shape of rules_java's `local_jdk`. Default bazel-diff and bazel-diff-fine
  aren't affected.
- **target-determinator keeps no snapshots.** It compares two revisions in one
  run, so it can't have cross-snapshot inconsistencies, and it selected nothing
  for an empty commit in any setup.

## The remote repo contents cache

- **Consistency.** With `--experimental_remote_repo_contents_cache`, every tool
  scores the same on all 47 scenarios as without it. Within one run, base and
  head see the same files, so bazel-diff-fine's dependence on what's on disk
  doesn't show up as a miss or an extra; it shows up across runs (see
  Determinism).
- **Downloads.** With the flag, 93–100% of every tool's downloads come from the
  remote cache. A cold first run of generate_hashes (query) downloads 389 MB in
  19s with the flag, versus 413 MB in 50s from the internet without it.
- **Building C++ fails with the flag**, with stock Bazel 9.2.0 as well as the
  custom Bazel: libc++ from the LLVM toolchain can't find its headers. The hash
  computations aren't affected; builds in the suite that run with the flag
  complete for Python, Go and Rust only. See [STATUS.md](STATUS.md).

## Summary by tool

- **generate_hashes (cquery)**: no misses, deterministic in every setup tested
  except a repo that writes its own absolute path into a file, about 1s per
  change. Over-selects when a file referenced by an external repo changes
  without changing outputs.
- **generate_hashes (query)**: the fastest tool, with the same determinism, and
  one blind spot: build configuration (`.bazelrc` flags), which an unconfigured
  query can't see.
- **target-determinator**: no misses, the best precision, immune to
  cross-snapshot inconsistency by design, and about 9 times slower per change
  than generate_hashes (cquery), because every run analyzes the base revision
  again. It needs both revisions in a local git checkout.
- **bazel-diff**: fast and deterministic, but misses every toolchain change and
  configuration changes.
- **bazel-diff-fine**: the same misses as bazel-diff, and its hashes depend on
  which external files are on disk, so it gives inconsistent results with the
  remote repo contents cache.
- **bazel-diff-cquery**: catches toolchain versions and configuration, misses
  toolchain patch edits and a tool version bump, over-selects as much as
  generate_hashes (cquery), takes about 5s per change, and its hashes depend on
  the output base path for a local repo with its path in an attribute.
