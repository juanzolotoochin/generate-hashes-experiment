# hashbench results

Stock Bazel 9.2.0 · bazel-diff v60.0.0 · target-determinator v0.34.0 · scale 0 · 39 base targets

| Tool | Exact | Scenarios with misses | Missed targets | Over-selected targets | Recall | Precision | Warm no-change | Warm per change (median) |
|---|---|---|---|---|---|---|---|---|
| custom | 24/42 | 6 | 50 | 34 | 65.3% | 73.4% | 0.2s | 0.2s |
| bazel-diff | 23/42 | 4 | 47 | 70 | 67.4% | 58.1% | 0.4s | 0.6s |
| target-determinator | 26/42 | 0 | 0 | 84 | 100.0% | 63.2% | 9.7s | 11.6s |

| Scenario | Category | Truth | custom | bazel-diff | target-determinator |
|---|---|---|---|---|---|
| control | control | 0 | ✅ | ✅ | ✅ |
| go_src_edit | source | 9 | +2 extra | +2 extra | +2 extra |
| go_src_comment | source | 9 | +2 extra | +2 extra | +2 extra |
| py_src_edit | source | 3 | ✅ | ✅ | ✅ |
| cc_header_edit | source | 3 | ✅ | ✅ | ✅ |
| rust_src_edit | source | 3 | ✅ | ✅ | ✅ |
| data_file_edit | source | 7 | ✅ | ✅ | ✅ |
| new_globbed_file | source | 6 | ✅ | ✅ | ✅ |
| unreferenced_file | source | 0 | ✅ | ✅ | ✅ |
| docs_edit | source | 0 | ✅ | ✅ | ✅ |
| chmod_runfile | source | 1 | ❌ 1 missed | ✅ | ✅ |
| chmod_source | source | 0 | ✅ | +11 extra | +11 extra |
| rename_source | source | 9 | +2 extra | +2 extra | +2 extra |
| build_add_dep | BUILD | 0 | +1 extra | +1 extra | +1 extra |
| build_copts | BUILD | 1 | ✅ | ✅ | ✅ |
| build_test_size | BUILD | 1 | ✅ | ✅ | ✅ |
| build_delete_target | BUILD | 0 | ✅ | ✅ | ✅ |
| build_comment | BUILD | 0 | ✅ | ✅ | ✅ |
| build_reorder | BUILD | 0 | ✅ | ✅ | ✅ |
| build_unused_load | BUILD | 0 | ✅ | ✅ | ✅ |
| build_visibility | BUILD | 0 | +11 extra | +11 extra | +11 extra |
| bzl_rule_impl | .bzl | 3 | ✅ | ✅ | ✅ |
| bzl_macro | .bzl | 1 | +2 extra | +2 extra | +2 extra |
| bzl_comment | .bzl | 0 | +3 extra | +3 extra | +3 extra |
| bzl_unused_fn | .bzl | 0 | +3 extra | +3 extra | +3 extra |
| go_dep_bump | dependency | 9 | +2 extra | +2 extra | +2 extra |
| pip_dep_bump | dependency | 3 | ✅ | ✅ | ✅ |
| rust_crate_bump | dependency | 3 | ✅ | ✅ | ✅ |
| multitool_bump | dependency | 2 | ❌ 2 missed | ✅ | ✅ |
| oci_digest_bump | dependency | 9 | +3 extra | +3 extra | +3 extra |
| img_digest_bump | dependency | 1 | ✅ | ✅ | ✅ |
| apko_package_add | dependency | 2 | ✅ | ✅ | ✅ |
| distroless_pkg_bump | dependency | 3 | +1 extra | +5 extra | +1 extra |
| distroless_snapshot_bump | dependency | 6 | +2 extra | +2 extra | +2 extra |
| distroless_timestamp | dependency | 0 | ✅ | +8 extra | ✅ |
| bzlmod_dep_bump | bzlmod | 0 | ✅ | +13 extra | ✅ |
| module_comment | bzlmod | 0 | ✅ | ✅ | ✅ |
| go_toolchain | toolchain | 16 | ❌ 16 missed | ❌ 16 missed | +2 extra |
| rust_toolchain | toolchain | 3 | ❌ 3 missed | ❌ 3 missed | ✅ |
| python_toolchain | toolchain | 3 | ✅ | ✅ | ✅ |
| llvm_toolchain | toolchain | 20 | ❌ 20 missed | ❌ 20 missed | +6 extra |
| bazelrc_copt | config | 8 | ❌ 8 missed | ❌ 8 missed | +31 extra |
