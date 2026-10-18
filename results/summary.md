# hashbench results

Stock Bazel 9.2.0 · bazel-diff v60.0.0 · target-determinator v0.34.0 · scale 0 · 43 base targets

| Tool | Exact | Scenarios with misses | Missed targets | Over-selected targets | Recall | Precision | Warm no-change | Warm per change (median) |
|---|---|---|---|---|---|---|---|---|
| generate_hashes (query) | 25/47 | 1 | 10 | 150 | 94.4% | 52.7% | 0.1s | 0.2s |
| generate_hashes (cquery) | 25/47 | 0 | 0 | 143 | 100.0% | 55.3% | 0.7s | 1.3s |
| bazel-diff | 23/47 | 5 | 60 | 90 | 66.1% | 56.5% | 0.5s | 0.7s |
| bazel-diff-fine | 23/47 | 5 | 57 | 93 | 67.8% | 56.3% | 0.6s | 0.8s |
| bazel-diff-cquery | 26/47 | 2 | 20 | 144 | 88.7% | 52.2% | 4.1s | 5.0s |
| target-determinator | 28/47 | 0 | 0 | 106 | 100.0% | 62.5% | 9.9s | 11.4s |
| generate_hashes (query)@remote | 25/47 | 1 | 10 | 150 | 94.4% | 52.7% | 0.1s | 0.2s |
| generate_hashes (cquery)@remote | 25/47 | 0 | 0 | 143 | 100.0% | 55.3% | 0.6s | 1.1s |
| bazel-diff@remote | 23/47 | 5 | 60 | 90 | 66.1% | 56.5% | 0.6s | 0.5s |
| bazel-diff-fine@remote | 23/47 | 5 | 57 | 93 | 67.8% | 56.3% | 0.7s | 0.8s |
| bazel-diff-cquery@remote | 26/47 | 2 | 20 | 144 | 88.7% | 52.2% | 4.6s | 4.6s |
| target-determinator@remote | 28/47 | 0 | 0 | 106 | 100.0% | 62.5% | 8.1s | 8.2s |

## Downloads

Bytes each tool's Bazel received over the network (internet and remote cache), starting from its own empty caches.

| Tool | Setup (base revision) | Setup time | All changes (total) |
|---|---|---|---|
| generate_hashes (query) | 413.4 MB | 50s | 281.0 MB |
| generate_hashes (cquery) | 820.4 MB | 69s | 662.3 MB |
| bazel-diff | 140.8 MB | 19s | 84.6 MB |
| bazel-diff-fine | 236.3 MB | 32s | 98.3 MB |
| bazel-diff-cquery | 828.7 MB | 68s | 729.7 MB |
| target-determinator | 820.4 MB | 80s | 662.3 MB |
| generate_hashes (query)@remote | 389.0 MB (97% remote) | 19s | 298.2 MB (100% remote) |
| generate_hashes (cquery)@remote | 407.5 MB (93% remote) | 21s | 467.0 MB (99% remote) |
| bazel-diff@remote | 401.6 MB (100% remote) | 14s | 1484.2 MB (100% remote) |
| bazel-diff-fine@remote | 414.7 MB (97% remote) | 28s | 781.6 MB (100% remote) |
| bazel-diff-cquery@remote | 462.9 MB (94% remote) | 29s | 1657.6 MB (100% remote) |
| target-determinator@remote | 421.6 MB (93% remote) | 42s | 1086.9 MB (100% remote) |

| Scenario | Category | Truth | generate_hashes (query) | generate_hashes (cquery) | bazel-diff | bazel-diff-fine | bazel-diff-cquery | target-determinator | generate_hashes (query)@remote | generate_hashes (cquery)@remote | bazel-diff@remote | bazel-diff-fine@remote | bazel-diff-cquery@remote | target-determinator@remote |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| control | control | 0 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| go_src_edit | source | 9 | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra |
| go_src_comment | source | 9 | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra |
| py_src_edit | source | 3 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| cc_header_edit | source | 3 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| rust_src_edit | source | 3 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| data_file_edit | source | 7 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| new_globbed_file | source | 6 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| unreferenced_file | source | 0 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| docs_edit | source | 0 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| chmod_runfile | source | 1 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| chmod_source | source | 0 | +11 extra | +11 extra | +11 extra | +11 extra | +11 extra | +11 extra | +11 extra | +11 extra | +11 extra | +11 extra | +11 extra | +11 extra |
| rename_source | source | 9 | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra |
| build_add_dep | BUILD | 0 | +1 extra | +1 extra | +1 extra | +1 extra | +1 extra | +1 extra | +1 extra | +1 extra | +1 extra | +1 extra | +1 extra | +1 extra |
| build_copts | BUILD | 1 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| build_test_size | BUILD | 1 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| build_delete_target | BUILD | 0 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| build_comment | BUILD | 0 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| build_reorder | BUILD | 0 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| build_unused_load | BUILD | 0 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| build_visibility | BUILD | 0 | +11 extra | +11 extra | +11 extra | +11 extra | +11 extra | +11 extra | +11 extra | +11 extra | +11 extra | +11 extra | +11 extra | +11 extra |
| bzl_rule_impl | .bzl | 3 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| bzl_macro | .bzl | 1 | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra |
| bzl_comment | .bzl | 0 | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra |
| bzl_unused_fn | .bzl | 0 | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra |
| go_dep_bump | dependency | 9 | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra |
| pip_dep_bump | dependency | 3 | ✅ | ✅ | +2 extra | +2 extra | ✅ | ✅ | ✅ | ✅ | +2 extra | +2 extra | ✅ | ✅ |
| rust_crate_bump | dependency | 3 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| multitool_bump | dependency | 2 | ✅ | ✅ | ✅ | ✅ | ❌ 2 missed | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ 2 missed | ✅ |
| oci_digest_bump | dependency | 9 | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra |
| img_digest_bump | dependency | 1 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| apko_package_add | dependency | 2 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| distroless_pkg_bump | dependency | 3 | +1 extra | +1 extra | +5 extra | +5 extra | +1 extra | +1 extra | +1 extra | +1 extra | +5 extra | +5 extra | +1 extra | +1 extra |
| distroless_snapshot_bump | dependency | 6 | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra | +2 extra |
| distroless_timestamp | dependency | 0 | ✅ | ✅ | +8 extra | +8 extra | ✅ | ✅ | ✅ | ✅ | +8 extra | +8 extra | ✅ | ✅ |
| bzlmod_dep_bump | bzlmod | 0 | +37 extra | +5 extra | +13 extra | +16 extra | +43 extra | ✅ | +37 extra | +5 extra | +13 extra | +16 extra | +43 extra | ✅ |
| module_comment | bzlmod | 0 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| bzlmod_patch_add | patch | 9 | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra |
| bzlmod_patch_edit | patch | 9 | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra | +3 extra |
| bzlmod_patch_noop | patch | 0 | +12 extra | +12 extra | +12 extra | +12 extra | ✅ | ✅ | +12 extra | +12 extra | +12 extra | +12 extra | ✅ | ✅ |
| toolchain_patch_edit | patch | 18 | +2 extra | +2 extra | ❌ 18 missed | ❌ 18 missed | ❌ 18 missed | +2 extra | +2 extra | +2 extra | ❌ 18 missed | ❌ 18 missed | ❌ 18 missed | +2 extra |
| toolchain_patch_noop | patch | 0 | +20 extra | +20 extra | ✅ | ✅ | ✅ | ✅ | +20 extra | +20 extra | ✅ | ✅ | ✅ | ✅ |
| go_toolchain | toolchain | 18 | +2 extra | +2 extra | ❌ 18 missed | ❌ 18 missed | +2 extra | +2 extra | +2 extra | +2 extra | ❌ 18 missed | ❌ 18 missed | +2 extra | +2 extra |
| rust_toolchain | toolchain | 3 | ✅ | ✅ | ❌ 3 missed | ❌ 3 missed | ✅ | ✅ | ✅ | ✅ | ❌ 3 missed | ❌ 3 missed | ✅ | ✅ |
| python_toolchain | toolchain | 5 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| llvm_toolchain | toolchain | 11 | +26 extra | +18 extra | ❌ 11 missed | ❌ 8 missed | +18 extra | +18 extra | +26 extra | +18 extra | ❌ 11 missed | ❌ 8 missed | +18 extra | +18 extra |
| bazelrc_copt | config | 10 | ❌ 10 missed | +33 extra | ❌ 10 missed | ❌ 10 missed | +30 extra | +33 extra | ❌ 10 missed | +33 extra | ❌ 10 missed | ❌ 10 missed | +30 extra | +33 extra |

## Consistency: same revision, different setups

Each tool's hashes of the base revision in local mode, compared with its hashes from a second checkout at another path with fresh output bases and caches at other paths (as on another CI machine or worktree), and with `--experimental_remote_repo_contents_cache` (where external repos are served lazily and are not on disk). A correct tool gives identical hashes; any difference means comparing hash snapshots from different machines or runs gives wrong answers.

| Tool | Compared with | Targets compared | Different hashes | Examples |
|---|---|---|---|---|
| generate_hashes (query) | remote repo contents cache on | 43 | 0 |  |
| generate_hashes (cquery) | remote repo contents cache on | 43 | 0 |  |
| bazel-diff | remote repo contents cache on | 102 | 0 |  |
| bazel-diff-fine | remote repo contents cache on | 102 | 8 | //py/app:app, //py/extra:extra, //py/extra:extra_test, //py/lib:greet_test, … and 4 more |

## Determinism: the same revision hashed twice

Each check hashes the same revision twice with one thing changed in between; every target whose hash differs would be selected although nothing changed. target-determinator keeps no snapshots: in each setup it compares the revision with an empty commit on top, and every target it selects is wrong. The checkout adds three hazard repos to the fixture, each with its own target: an absolute symlink (//hazard:symlink), a local repo with its path in a string attribute like local_jdk (//hazard:path_attr), and a fetched repo with its path in a BUILD file comment (//hazard:path_content). Custom Bazel: bazel-hashes-9-2, 2026-10-18T10:38:50+02:00.

| Check | generate_hashes (query) | generate_hashes (cquery) | bazel-diff | bazel-diff-fine | target-determinator | bazel-diff-cquery | What changes |
|---|---|---|---|---|---|---|---|
| setup | – | – | – | – | ✅ | – | First snapshot. |
| repeat | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Hash twice in a row with the same Bazel server. |
| restart | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Shut down the Bazel server between the two snapshots. |
| environment | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Restart the server with a different HOME, USER and TMPDIR. |
| output_base_path | ❌ 1 of 46 | ❌ 1 of 46 | ✅ | ✅ | ✅ | ❌ 2 of 109 | Second snapshot from an output base at another path (same checkout and caches), as on another machine. |
| fresh_output_base | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Delete the output base between the two snapshots (same path, caches kept). |
| checkout_path | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Second snapshot from a clone of the checkout at another path (output base recreated at the same path), as in another CI worktree. |
| fresh_caches | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Delete the output base and the repository and repo contents caches between the two snapshots (same paths). |
| build_between | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Build //... between the two snapshots. |
| remote_cache | ✅ | ✅ | ✅ | ❌ 8 of 109 | ✅ | ✅ | First snapshot with --experimental_remote_repo_contents_cache, second without it (output base recreated at the same path). |
| remote_off_same_output_base | ✅ | ✅ | ✅ | ❌ 8 of 109 | ✅ | ✅ | First snapshot with --experimental_remote_repo_contents_cache, second without it in the same output base (as when toggling the flag in .bazelrc). |
| remote_on_again | ✅ | ✅ | ✅ | ❌ 8 of 109 | ✅ | ✅ | Then with --experimental_remote_repo_contents_cache again, same output base, compared with the first snapshot. |
| build_between_remote | ✅ (build errors) | ✅ (build errors) | ✅ (build errors) | ❌ 5 of 109 (build errors) | ✅ (build errors) | ✅ (build errors) | With --experimental_remote_repo_contents_cache: build //... between the two snapshots. |
| remote_then_local_build | ✅ | ✅ | ✅ | ❌ 8 of 109 | ✅ | ✅ | First snapshot with --experimental_remote_repo_contents_cache (as in CI), second without it after a local build //... (as on a laptop), same output base path. |

- output_base_path, generate_hashes (query): //hazard:path_content
- output_base_path, generate_hashes (cquery): //hazard:path_content
- output_base_path, bazel-diff-cquery: //hazard:path_attr, //hazard:path_attr.txt
- remote_cache, bazel-diff-fine: //py/app:app, //py/extra:extra, //py/extra:extra_test, //py/lib:greet_test, //py/lib:lib, //rust/app:app, //rust/lib:lib, //rust/lib:lib_test
- remote_off_same_output_base, bazel-diff-fine: //py/app:app, //py/extra:extra, //py/extra:extra_test, //py/lib:greet_test, //py/lib:lib, //rust/app:app, //rust/lib:lib, //rust/lib:lib_test
- remote_on_again, bazel-diff-fine: //py/app:app, //py/extra:extra, //py/extra:extra_test, //py/lib:greet_test, //py/lib:lib, //rust/app:app, //rust/lib:lib, //rust/lib:lib_test
- build_between_remote, bazel-diff-fine: //py/app:app, //py/extra:extra, //py/extra:extra_test, //py/lib:greet_test, //py/lib:lib
- remote_then_local_build, bazel-diff-fine: //py/app:app, //py/extra:extra, //py/extra:extra_test, //py/lib:greet_test, //py/lib:lib, //rust/app:app, //rust/lib:lib, //rust/lib:lib_test

## Repo contents cache support by module

Every repository the tools fetched for the base revision. Only repositories whose rule declares them reproducible are cached (locally and by `--experimental_remote_repo_contents_cache`); the rest are fetched from the internet even with a remote cache. **Bold**: a direct `bazel_dep` of the fixture.

| Module | Cacheable repos | Size | Not cacheable | Size | Not cacheable repos |
|---|---|---|---|---|---|
| **rules_rs** | 12 | 1.1 GB | 2 | 0 MB | rules_rs++crate+crates, rules_rs++crate+crates_https___index.crates.io_ |
| **llvm** | 9 | 990 MB | 2 | 0 MB | llvm++glibc+glibc, llvm++kernel_headers+kernel_headers |
| **rules_python** | 4 | 362 MB | 4 | 0 MB | rules_python++config+rules_python_internal, rules_python++pip+pypi, rules_python++python+pythons_hub, rules_python++uv+uv |
| **rules_go** | 2 | 254 MB | 3 | 0 MB | rules_go++go_sdk+go_host_compatible_sdk_label, rules_go++go_sdk+go_toolchains, rules_go++go_sdk+io_bazel_rules_nogo |
| **gazelle** | 4 | 37 MB | 2 | 154 MB | gazelle++go_deps+bazel_gazelle_go_repository_config, gazelle++non_module_deps+bazel_gazelle_go_repository_cache |
| **rules_apko** | 14 | 50 MB | 1 | 11 MB | rules_apko++apko+wolfi_base_packages.wolfi.dev_os_x86_64_x86_64 |
| protobuf | 1 | 45 MB | 0 | 0 |  |
| **rules_img** | 2 | 27 MB | 1 | 0 MB | rules_img++prebuilt_img_tool+img_toolchain |
| rules_kotlin | 1 | 25 MB | 1 | 0 MB | rules_kotlin++rules_kotlin_extensions+com_github_jetbrains_kotlin |
| gawk | 1 | 24 MB | 0 | 0 |  |
| **rules_oci** | 1 | 9 MB | 5 | 12 MB | rules_oci++oci+distroless_static, rules_oci++oci+distroless_static_linux_amd64, rules_oci++oci+oci_crane_toolchains, rules_oci++oci+oci_regctl_linux_amd64, … and 1 more |
| bazel_lib | 5 | 17 MB | 5 | 0 MB | bazel_lib++toolchains+copy_directory_toolchains, bazel_lib++toolchains+copy_to_directory_toolchains, bazel_lib++toolchains+coreutils_toolchains, bazel_lib++toolchains+expand_template_toolchains, … and 1 more |
| rules_apple | 1 | 8 MB | 0 | 0 |  |
| rules_jvm_external | 1 | 7 MB | 0 | 0 |  |
| **rules_distroless** | 9 | 7 MB | 1 | 0 MB | rules_distroless++apt+bookworm |
| aspect_bazel_lib | 2 | 1 MB | 10 | 3 MB | aspect_bazel_lib++toolchains+bsd_tar_toolchains, aspect_bazel_lib++toolchains+copy_directory_toolchains, aspect_bazel_lib++toolchains+copy_to_directory_toolchains, aspect_bazel_lib++toolchains+coreutils_toolchains, … and 6 more |
| zlib | 1 | 4 MB | 0 | 0 |  |
| googletest | 1 | 4 MB | 0 | 0 |  |
| apple_support | 2 | 4 MB | 0 | 0 |  |
| bazel_tools | 1 | 4 MB | 2 | 0 MB | bazel_tools+winsdk_configure+local_config_winsdk, bazel_tools+xcode_configure_extension+local_config_xcode |
| **rules_cc** | 2 | 3 MB | 1 | 0 MB | rules_cc++compatibility_proxy+cc_compatibility_proxy |
| **rules_multitool** | 1 | 0 MB | 2 | 2 MB | rules_multitool++multitool+multitool, rules_multitool++multitool+multitool.jq.linux_x86_64 |
| **tar.bzl** | 2 | 2 MB | 1 | 0 MB | tar.bzl++toolchains+bsd_tar_toolchains |
| rules_swift | 1 | 2 MB | 1 | 0 MB | rules_swift++non_module_deps+build_bazel_rules_swift_local_config |
| rules_java | 1 | 1 MB | 39 | 0 MB | rules_java++compatibility_proxy+compatibility_proxy, rules_java++toolchains+local_jdk, rules_java++toolchains+remote_jdk8_linux_aarch64_toolchain_config_repo, rules_java++toolchains+remote_jdk8_linux_s390x_toolchain_config_repo, … and 35 more |
| package_metadata | 1 | 0 MB | 0 | 0 |  |
| toml.bzl | 1 | 0 MB | 0 | 0 |  |
| windows_support | 1 | 0 MB | 0 | 0 |  |
| rules_runfiles_group | 1 | 0 MB | 0 | 0 |  |
| rules_license | 1 | 0 MB | 0 | 0 |  |
| **bazel_skylib** | 1 | 0 MB | 0 | 0 |  |
| with_cfg.bzl | 1 | 0 MB | 0 | 0 |  |
| yq.bzl | 1 | 0 MB | 1 | 0 MB | yq.bzl++yq+yq_toolchains |
| jq.bzl | 1 | 0 MB | 0 | 0 |  |
| stardoc | 1 | 0 MB | 0 | 0 |  |
| **rules_shell** | 1 | 0 MB | 1 | 0 MB | rules_shell++sh_configure+local_config_shell |
| (fixture) | 1 | 0 MB | 0 | 0 |  |
| buildifier_prebuilt | 1 | 0 MB | 1 | 0 MB | buildifier_prebuilt++buildifier_prebuilt_deps_extension+buildifier_prebuilt_toolchains |
| bazel_features | 1 | 0 MB | 2 | 0 MB | bazel_features++version_extension+bazel_features_globals, bazel_features++version_extension+bazel_features_version |
| hermetic_launcher | 1 | 0 MB | 0 | 0 |  |
| sha256.bzl | 1 | 0 MB | 0 | 0 |  |
| **platforms** | 1 | 0 MB | 1 | 0 MB | platforms+host_platform+host_platform |
| buildozer | 1 | 0 MB | 0 | 0 |  |
| pybind11_bazel | 1 | 0 MB | 0 | 0 |  |

### Downloads per scenario

| Scenario | generate_hashes (query) | generate_hashes (cquery) | bazel-diff | bazel-diff-fine | bazel-diff-cquery | target-determinator | generate_hashes (query)@remote | generate_hashes (cquery)@remote | bazel-diff@remote | bazel-diff-fine@remote | bazel-diff-cquery@remote | target-determinator@remote |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| control | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| go_src_edit | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| go_src_comment | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| py_src_edit | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| cc_header_edit | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| rust_src_edit | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| data_file_edit | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| new_globbed_file | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| unreferenced_file | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| docs_edit | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| chmod_runfile | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| chmod_source | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| rename_source | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| build_add_dep | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| build_copts | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| build_test_size | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| build_delete_target | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| build_comment | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| build_reorder | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| build_unused_load | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| build_visibility | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| bzl_rule_impl | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| bzl_macro | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| bzl_comment | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| bzl_unused_fn | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.0 MB (100% remote) |
| go_dep_bump | 0.0 MB | 0.0 MB | 0.1 MB | 0.0 MB | 0.1 MB | 0.0 MB | 0.0 MB (100% remote) | 0.0 MB (100% remote) | 0.1 MB (3% remote) | 0.0 MB (100% remote) | 6.9 MB (99% remote) | 14.1 MB (100% remote) |
| pip_dep_bump | 0.0 MB | 0.0 MB | 0.1 MB | 0.0 MB | 0.1 MB | 0.0 MB | 0.0 MB (100% remote) | 21.7 MB (100% remote) | 0.1 MB (3% remote) | 0.0 MB (100% remote) | 7.9 MB (99% remote) | 71.7 MB (100% remote) |
| rust_crate_bump | 0.0 MB | 0.0 MB | 0.1 MB | 0.0 MB | 0.1 MB | 0.0 MB | 0.0 MB (100% remote) | 21.7 MB (100% remote) | 0.1 MB (3% remote) | 0.0 MB (100% remote) | 7.9 MB (99% remote) | 50.0 MB (100% remote) |
| multitool_bump | 0.0 MB | 2.3 MB | 0.1 MB | 0.0 MB | 2.3 MB | 2.3 MB | 0.0 MB (100% remote) | 16.4 MB (86% remote) | 0.1 MB (3% remote) | 0.0 MB (100% remote) | 9.1 MB (75% remote) | 30.5 MB (93% remote) |
| oci_digest_bump | 0.0 MB | 0.0 MB | 0.1 MB | 0.0 MB | 0.1 MB | 0.0 MB | 19.7 MB (100% remote) | 21.7 MB (100% remote) | 16.7 MB (100% remote) | 17.4 MB (100% remote) | 23.8 MB (100% remote) | 71.7 MB (100% remote) |
| img_digest_bump | 0.0 MB | 0.0 MB | 0.1 MB | 0.0 MB | 0.1 MB | 0.0 MB | 19.8 MB (100% remote) | 21.7 MB (100% remote) | 16.7 MB (100% remote) | 17.4 MB (100% remote) | 23.8 MB (100% remote) | 57.6 MB (100% remote) |
| apko_package_add | 0.5 MB | 0.5 MB | 0.1 MB | 0.5 MB | 0.6 MB | 0.5 MB | 19.8 MB (100% remote) | 21.7 MB (100% remote) | 16.7 MB (100% remote) | 17.5 MB (100% remote) | 23.8 MB (100% remote) | 35.9 MB (100% remote) |
| distroless_pkg_bump | 0.3 MB | 0.3 MB | 0.1 MB | 0.3 MB | 0.4 MB | 0.3 MB | 0.0 MB (100% remote) | 14.1 MB (100% remote) | 0.1 MB (3% remote) | 0.0 MB (100% remote) | 6.9 MB (99% remote) | 28.3 MB (100% remote) |
| distroless_snapshot_bump | 6.6 MB | 6.6 MB | 0.1 MB | 6.6 MB | 6.7 MB | 6.6 MB | 0.0 MB (100% remote) | 14.1 MB (100% remote) | 0.1 MB (3% remote) | 0.0 MB (100% remote) | 6.9 MB (99% remote) | 28.3 MB (100% remote) |
| distroless_timestamp | 6.9 MB | 6.9 MB | 0.1 MB | 6.9 MB | 7.0 MB | 6.9 MB | 0.0 MB (100% remote) | 14.1 MB (100% remote) | 0.1 MB (3% remote) | 0.0 MB (100% remote) | 6.9 MB (99% remote) | 28.3 MB (100% remote) |
| bzlmod_dep_bump | 0.1 MB | 0.1 MB | 0.2 MB | 0.2 MB | 0.2 MB | 0.1 MB | 19.7 MB (100% remote) | 21.7 MB (100% remote) | 16.7 MB (100% remote) | 17.5 MB (100% remote) | 23.8 MB (100% remote) | 71.7 MB (100% remote) |
| module_comment | 0.0 MB | 0.0 MB | 0.1 MB | 0.1 MB | 0.1 MB | 0.0 MB | 19.7 MB (100% remote) | 21.7 MB (100% remote) | 16.7 MB (100% remote) | 17.5 MB (100% remote) | 23.8 MB (100% remote) | 57.6 MB (100% remote) |
| bzlmod_patch_add | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 19.7 MB (100% remote) | 21.7 MB (100% remote) | 17.3 MB (100% remote) | 17.4 MB (100% remote) | 24.3 MB (100% remote) | 43.4 MB (100% remote) |
| bzlmod_patch_edit | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 19.7 MB (100% remote) | 21.7 MB (100% remote) | 16.7 MB (100% remote) | 17.4 MB (100% remote) | 23.7 MB (100% remote) | 43.4 MB (100% remote) |
| bzlmod_patch_noop | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.2 MB (100% remote) | 21.7 MB (100% remote) | 0.8 MB (100% remote) | 0.2 MB (100% remote) | 8.7 MB (100% remote) | 43.4 MB (100% remote) |
| toolchain_patch_edit | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 1.8 MB (100% remote) | 23.3 MB (100% remote) | 0.2 MB (100% remote) | 3.4 MB (100% remote) | 9.6 MB (100% remote) | 23.3 MB (100% remote) |
| toolchain_patch_noop | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 0.0 MB | 1.6 MB (100% remote) | 1.6 MB (100% remote) | 0.0 MB (100% remote) | 3.1 MB (100% remote) | 1.6 MB (100% remote) | 3.2 MB (100% remote) |
| go_toolchain | 70.7 MB | 70.7 MB | 70.7 MB | 70.7 MB | 70.7 MB | 70.7 MB | 21.3 MB (100% remote) | 23.3 MB (100% remote) | 262.9 MB (100% remote) | 20.5 MB (100% remote) | 271.5 MB (100% remote) | 62.2 MB (100% remote) |
| rust_toolchain | 12.1 MB | 229.6 MB | 12.1 MB | 12.1 MB | 229.7 MB | 229.6 MB | 64.7 MB (100% remote) | 66.7 MB (100% remote) | 306.6 MB (100% remote) | 64.0 MB (100% remote) | 315.2 MB (100% remote) | 121.3 MB (100% remote) |
| python_toolchain | 0.1 MB | 123.6 MB | 0.1 MB | 0.1 MB | 123.6 MB | 123.6 MB | 21.4 MB (100% remote) | 23.3 MB (100% remote) | 263.2 MB (100% remote) | 20.6 MB (100% remote) | 271.8 MB (100% remote) | 77.9 MB (100% remote) |
| llvm_toolchain | 183.6 MB | 221.6 MB | 0.7 MB | 0.7 MB | 288.1 MB | 221.6 MB | 24.4 MB (100% remote) | 26.4 MB (100% remote) | 266.3 MB (100% remote) | 274.6 MB (100% remote) | 280.1 MB (100% remote) | 81.0 MB (100% remote) |
| bazelrc_copt | 0.0 MB | 0.0 MB | 0.1 MB | 0.1 MB | 0.1 MB | 0.0 MB | 24.4 MB (100% remote) | 26.4 MB (100% remote) | 266.3 MB (100% remote) | 273.1 MB (100% remote) | 279.5 MB (100% remote) | 42.1 MB (100% remote) |
