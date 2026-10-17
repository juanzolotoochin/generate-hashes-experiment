# Status

## Done

- Harness, fixture (39 targets), 42 scenarios and the build-graph oracle are
  complete. See the README.
- Correctness results for all 42 scenarios are in `results/`, from a run in
  which every target built (no excluded targets).
- `custom-cquery` (the custom Bazel's `cquery --generate_hashes`) was added and
  scored with `-reuse-truth`, merged into `results/`.
- The experimental hermetic sandbox
  (`--experimental_use_hermetic_linux_sandbox`) was removed. It hid the host's
  glibc, which prebuilt toolchains (the official `rustc`, the Python
  interpreter) need, and it only affected the oracle's build, not the tools.
  Hermeticity comes from pinned toolchains, no host tools in rules, a scrubbed
  `PATH`, and the pinned static bash as `--shell_executable`.

## Next steps

1. **Scaling sweep:** run
   `-scale=0,2000,8000,16000 -tools=custom,custom-cquery,bazel-diff`, then publish
   `scaling.md` and `scaling.html` under `results/`. Expect about an hour.
2. **Custom Bazel:** link the fork and its build instructions in the README
   (there's a TODO).
3. Optional:
   - Remove or document the `HASHBENCH_ORACLE_DEBUG` fingerprint dump in
     `internal/oracle/oracle.go`.
   - Run target-determinator in the sweep at smaller sizes.
