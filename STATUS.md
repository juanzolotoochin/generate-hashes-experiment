# Status

`results/` is current (October 2026): five tools, 47 scenarios, with the remote
repo contents cache off and on, using a bundled localhost cache.

## Open issues

- `query --generate_hashes` misses edits to the contents of a toolchain's patch
  file (`toolchain_patch_edit`), because toolchain repos are hashed by their
  definitions. The suggested fix is to include the digests of files a repo's
  definition refers to (patches, `build_file`, ...), which Bazel already
  records. Reported to the fork.
- BuildBuddy: `llvm-project` misses its remote repo contents cache but hits with
  a local cache. Not investigated further.
- rules_distroless is held at 0.8.0. Migrating to 0.9 (new `apt` API) needs new
  lock data for the three distroless scenarios.

## Next steps

1. Rerun with the next fork build once the patch-content fix lands.
2. Scaling sweep: `-scale=0,2000,8000,16000 -tools=custom,custom-cquery,bazel-diff`.
3. Link the custom Bazel fork and its build instructions in the README.
