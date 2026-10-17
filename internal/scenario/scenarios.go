package scenario

// Scenario is one change applied on top of the base revision.
type Scenario struct {
	Name        string
	Category    string
	Description string
	// Expectation describes, for readers, what a perfect tool should report.
	// The authoritative answer is always the build oracle.
	Expectation string
	Ops         []Op
}

const (
	digestStatic12  = "sha256:d75cdd72874d4790092fcb1b058493ecf6bb5bf2b2b897045b00ff01d91843f2"
	digestStatic12b = "sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab"
	digestBase12    = "sha256:fabbf1c0c357a3d42550111351daed089b20a2c954df13ee2fcff60602515e84"
	digestBase12b   = "sha256:7f0c72cd138b442ae0deeb69c08b1acf5525439ba251a49ad93c320a061567e5"
)

// All returns every scenario, in execution order. Cheap local edits come
// first; dependency and toolchain changes, which refetch external repos, last.
func All() []Scenario {
	return []Scenario{
		{
			Name: "control", Category: "control",
			Description: "Empty commit: nothing changes.",
			Expectation: "Nothing affected.",
		},

		// ---- Source files ----
		{
			Name: "go_src_edit", Category: "source",
			Description: "Add a function to go/greet/greet.go.",
			Expectation: "//go/greet and everything that links it (greeter, its images, greet_test).",
			Ops:         []Op{Append{"go/greet/greet.go", "\nfunc unused() int { return 42 }\n"}},
		},
		{
			Name: "go_src_comment", Category: "source",
			Description: "Comment-only edit to go/greet/greet.go.",
			Expectation: "Compiled outputs are identical, so ideally nothing; content-hashing tools flag the go/greet closure.",
			Ops:         []Op{Append{"go/greet/greet.go", "\n// A comment.\n"}},
		},
		{
			Name: "py_src_edit", Category: "source",
			Description: "Add a function to py/lib/greet.py.",
			Expectation: "//py/lib, its test and //py/app (sources ship in runfiles).",
			Ops:         []Op{Append{"py/lib/greet.py", "\n\ndef _unused():\n    return 42\n"}},
		},
		{
			Name: "cc_header_edit", Category: "source",
			Description: "Add an #include to cc/lib/hello.h.",
			Expectation: "//cc/lib targets and //cc/app.",
			Ops:         []Op{Replace{"cc/lib/hello.h", "#include <string>\n", "#include <string>\n#include <vector>\n"}},
		},
		{
			Name: "rust_src_edit", Category: "source",
			Description: "Add a function to rust/lib/lib.rs.",
			Expectation: "//rust/lib, its test and //rust/app.",
			Ops:         []Op{Append{"rust/lib/lib.rs", "\npub fn unused() -> u32 {\n    42\n}\n"}},
		},
		{
			Name: "data_file_edit", Category: "source",
			Description: "Edit data/a.json, consumed through a glob() filegroup.",
			Expectation: "Everything built from //data:json or //data:a.json: the bundles, :shout, :summary and :check_test.",
			Ops:         []Op{Replace{"data/a.json", `"a"`, `"a2"`}},
		},
		{
			Name: "new_globbed_file", Category: "source",
			Description: "Add data/c.json, picked up by glob([\"*.json\"]).",
			Expectation: "Consumers of //data:json; not //tools:shout (depends on a.json only).",
			Ops:         []Op{Write{"data/c.json", "{\"name\": \"c\"}\n"}},
		},
		{
			Name: "unreferenced_file", Category: "source",
			Description: "Add go/greet/NOTES.txt, which no target references.",
			Expectation: "Nothing affected.",
			Ops:         []Op{Write{"go/greet/NOTES.txt", "notes\n"}},
		},
		{
			Name: "docs_edit", Category: "source",
			Description: "Edit docs/README.md, which no target references.",
			Expectation: "Nothing affected.",
			Ops:         []Op{Append{"docs/README.md", "\nMore docs.\n"}},
		},
		{
			Name: "chmod_runfile", Category: "source",
			Description: "Add the exec bit to tools/fixture.txt, a data file in //tools:check_test's runfiles.",
			Expectation: "//tools:check_test (a file it ships changes mode).",
			Ops:         []Op{Chmod{"tools/fixture.txt", 0o755}},
		},
		{
			Name: "chmod_source", Category: "source",
			Description: "Add the exec bit to go/greet/greet.go.",
			Expectation: "Nothing affected (compiled outputs don't depend on source modes).",
			Ops:         []Op{Chmod{"go/greet/greet.go", 0o755}},
		},
		{
			Name: "rename_source", Category: "source",
			Description: "Rename go/greet/greet.go to greeting.go and update srcs.",
			Expectation: "Ideally nothing beyond what embeds file names; tools typically flag the go/greet closure.",
			Ops: []Op{
				Rename{"go/greet/greet.go", "go/greet/greeting.go"},
				Replace{"go/greet/BUILD.bazel", `srcs = ["greet.go"]`, `srcs = ["greeting.go"]`},
			},
		},

		// ---- BUILD files ----
		{
			Name: "build_add_dep", Category: "BUILD",
			Description: "Add @pypi//six as a direct dep of //py/app.",
			Expectation: "Runfiles are unchanged (six was already transitive), so ideally nothing; //py/app at most.",
			Ops:         []Op{Replace{"py/app/BUILD.bazel", `    deps = ["//py/lib"],`, "    deps = [\n        \"//py/lib\",\n        \"@pypi//six\",\n    ],"}},
		},
		{
			Name: "build_copts", Category: "BUILD",
			Description: "Add copts = [\"-O2\"] to //cc/app.",
			Expectation: "//cc/app only.",
			Ops:         []Op{Replace{"cc/app/BUILD.bazel", `    srcs = ["main.cc"],`, "    srcs = [\"main.cc\"],\n    copts = [\"-O2\"],"}},
		},
		{
			Name: "build_test_size", Category: "BUILD",
			Description: "Set size = \"small\" on //go/greet:greet_test.",
			Expectation: "//go/greet:greet_test only (test attribute; outputs unchanged).",
			Ops:         []Op{Replace{"go/greet/BUILD.bazel", `    embed = [":greet"],`, "    embed = [\":greet\"],\n    size = \"small\","}},
		},
		{
			Name: "build_delete_target", Category: "BUILD",
			Description: "Delete //go/greet:greet_test.",
			Expectation: "One target removed; nothing else affected.",
			Ops:         []Op{Replace{"go/greet/BUILD.bazel", "\ngo_test(\n    name = \"greet_test\",\n    srcs = [\"greet_test.go\"],\n    embed = [\":greet\"],\n)\n", ""}},
		},
		{
			Name: "build_comment", Category: "BUILD",
			Description: "Add comments and blank lines to go/greet/BUILD.bazel.",
			Expectation: "Nothing affected.",
			Ops:         []Op{Replace{"go/greet/BUILD.bazel", "go_library(\n", "# A comment.\n\n\ngo_library(  # trailing comment\n"}},
		},
		{
			Name: "build_reorder", Category: "BUILD",
			Description: "Reorder go_library kwargs (same values).",
			Expectation: "Nothing affected.",
			Ops: []Op{Replace{"go/greet/BUILD.bazel",
				"    srcs = [\"greet.go\"],\n    importpath = \"example.com/hashbench/go/greet\",\n",
				"    importpath = \"example.com/hashbench/go/greet\",\n    srcs = [\"greet.go\"],\n"}},
		},
		{
			Name: "build_unused_load", Category: "BUILD",
			Description: "Add an unused load() to go/greet/BUILD.bazel.",
			Expectation: "Nothing affected.",
			Ops:         []Op{Prepend{"go/greet/BUILD.bazel", "load(\"@bazel_skylib//lib:paths.bzl\", \"paths\")  # unused\n"}},
		},
		{
			Name: "build_visibility", Category: "BUILD",
			Description: "Narrow //go/greet visibility.",
			Expectation: "Outputs unchanged, so nothing; tools typically flag the go/greet closure.",
			Ops:         []Op{Replace{"go/greet/BUILD.bazel", `visibility = ["//visibility:public"]`, `visibility = ["//go:__subpackages__"]`}},
		},

		// ---- .bzl files ----
		{
			Name: "bzl_rule_impl", Category: ".bzl",
			Description: "Change the text_bundle rule implementation (adds a blank line after the header).",
			Expectation: "//tools:notes, :pair_a, :pair_b; not :shout or :summary (rules in another file).",
			Ops:         []Op{Replace{"tools/defs.bzl", "    args.add(\"bundle\")\n", "    args.add(\"bundle\")\n    args.add(\"--blank\")\n"}},
		},
		{
			Name: "bzl_macro", Category: ".bzl",
			Description: "Change the header the bundle_pair macro passes to its second target.",
			Expectation: "//tools:pair_b only.",
			Ops:         []Op{Replace{"tools/defs.bzl", `header = "# b", **kwargs)`, `header = "# B", **kwargs)`}},
		},
		{
			Name: "bzl_comment", Category: ".bzl",
			Description: "Add a comment to tools/defs.bzl.",
			Expectation: "Nothing affected.",
			Ops:         []Op{Prepend{"tools/defs.bzl", "# A comment.\n"}},
		},
		{
			Name: "bzl_unused_fn", Category: ".bzl",
			Description: "Add an unused function to tools/defs.bzl.",
			Expectation: "Nothing affected.",
			Ops:         []Op{Append{"tools/defs.bzl", "\ndef _unused():\n    return 42\n"}},
		},

		// ---- Third-party dependencies ----
		{
			Name: "go_dep_bump", Category: "dependency",
			Description: "github.com/google/uuid v1.6.0 -> v1.5.0 (go.mod + go.sum).",
			Expectation: "Go targets linking uuid: //go/greet closure and the images.",
			Ops:         []Op{CopyData{"go.mod", "go_dep/go.mod.fixture"}, CopyData{"go.sum", "go_dep/go.sum"}},
		},
		{
			Name: "pip_dep_bump", Category: "dependency",
			Description: "six 1.17.0 -> 1.16.0 in the hashed requirements lock.",
			Expectation: "Python targets depending on six (//py/...).",
			Ops:         []Op{CopyData{"py/requirements_lock.txt", "pip_dep/requirements_lock.txt"}},
		},
		{
			Name: "rust_crate_bump", Category: "dependency",
			Description: "itoa 1.0.18 -> 1.0.17 in Cargo.lock.",
			Expectation: "//rust/... targets.",
			Ops:         []Op{CopyData{"Cargo.lock", "rust_crate/Cargo.lock"}},
		},
		{
			Name: "multitool_bump", Category: "dependency",
			Description: "rules_multitool: jq 1.7.1 -> 1.8.1 in the lockfile.",
			Expectation: "Targets using jq: //tools:summary and //tools:check_test.",
			Ops:         []Op{CopyData{"tools/multitool.lock.json", "multitool/multitool.lock.json"}},
		},
		{
			Name: "oci_digest_bump", Category: "dependency",
			Description: "rules_oci: change the oci.pull digest of distroless/static.",
			Expectation: "Images built on @distroless_static: //oci:image and both //distroless images.",
			Ops:         []Op{Replace{"MODULE.bazel", digestStatic12, digestStatic12b}},
		},
		{
			Name: "img_digest_bump", Category: "dependency",
			Description: "rules_img: change the pulled base image digest.",
			Expectation: "//img:image only.",
			Ops:         []Op{Replace{"MODULE.bazel", digestBase12, digestBase12b}},
		},
		{
			Name: "apko_package_add", Category: "dependency",
			Description: "rules_apko: add jq to apko.yaml and re-lock (new packages jq, oniguruma).",
			Expectation: "//apko:image only.",
			Ops:         []Op{CopyData{"apko/apko.yaml", "apko/apko.yaml"}, CopyData{"apko/apko.lock.json", "apko/apko.lock.json"}},
		},
		{
			Name: "distroless_pkg_bump", Category: "dependency",
			Description: "rules_distroless: tzdata 2024b -> 2025b in the apt lock (only that package).",
			Expectation: "//distroless:tzdata_image only.",
			Ops:         []Op{CopyData{"distroless/bookworm.lock.json", "distroless_pkg/bookworm.lock.json"}},
		},
		{
			Name: "distroless_snapshot_bump", Category: "dependency",
			Description: "rules_distroless: move the Debian snapshot four months (tzdata, libc6, openssl, libgcc change).",
			Expectation: "Both //distroless images.",
			Ops:         []Op{CopyData{"distroless/bookworm.yaml", "distroless_snapshot/bookworm.yaml"}, CopyData{"distroless/bookworm.lock.json", "distroless_snapshot/bookworm.lock.json"}},
		},
		{
			Name: "distroless_timestamp", Category: "dependency",
			Description: "rules_distroless: move the snapshot 12h; every package is byte-identical, only URLs change.",
			Expectation: "Nothing affected.",
			Ops:         []Op{CopyData{"distroless/bookworm.yaml", "distroless_timestamp/bookworm.yaml"}, CopyData{"distroless/bookworm.lock.json", "distroless_timestamp/bookworm.lock.json"}},
		},

		// ---- Module graph ----
		{
			Name: "bzlmod_dep_bump", Category: "bzlmod",
			Description: "tar.bzl 0.10.8 -> 0.10.9.",
			Expectation: "Targets built with tar.bzl rules (//oci:greeter_layer, //oci:image), if the bump changes their outputs.",
			Ops:         []Op{Replace{"MODULE.bazel", `bazel_dep(name = "tar.bzl", version = "0.10.8")`, `bazel_dep(name = "tar.bzl", version = "0.10.9")`}},
		},
		{
			Name: "module_comment", Category: "bzlmod",
			Description: "Add a comment to MODULE.bazel.",
			Expectation: "Nothing affected.",
			Ops:         []Op{Prepend{"MODULE.bazel", "# A comment.\n"}},
		},

		// ---- Toolchains ----
		{
			Name: "go_toolchain", Category: "toolchain",
			Description: "Go SDK 1.27.2 -> 1.27.1.",
			Expectation: "Every Go target (compiled outputs change) and the images containing them.",
			Ops:         []Op{Replace{"MODULE.bazel", `go_sdk.download(version = "1.27.2")`, `go_sdk.download(version = "1.27.1")`}},
		},
		{
			Name: "rust_toolchain", Category: "toolchain",
			Description: "Rust toolchain 1.97.1 -> 1.97.0.",
			Expectation: "Every Rust target.",
			Ops:         []Op{Replace{"MODULE.bazel", `    version = "1.97.1",`, `    version = "1.97.0",`}},
		},
		{
			Name: "python_toolchain", Category: "toolchain",
			Description: "Python 3.12 -> 3.13 (interpreter and pip hub).",
			Expectation: "Every Python target.",
			Ops:         []Op{Replace{"MODULE.bazel", `    python_version = "3.12",`, `    python_version = "3.13",`}},
		},
		{
			Name: "llvm_toolchain", Category: "toolchain",
			Description: "Hermetic LLVM toolchain module 0.8.19 -> 0.8.18.",
			Expectation: "Every C/C++ target (and anything else the toolchain compiles or links).",
			Ops:         []Op{Replace{"MODULE.bazel", `bazel_dep(name = "llvm", version = "0.8.19")`, `bazel_dep(name = "llvm", version = "0.8.18")`}},
		},

		// ---- Build configuration ----
		{
			Name: "bazelrc_copt", Category: "config",
			Description: "Add build --copt=-DHASHBENCH=1 to .bazelrc.",
			Expectation: "Every C/C++ compile (outputs change); invisible to unconfigured query graphs.",
			Ops:         []Op{Append{".bazelrc", "build --copt=-DHASHBENCH=1\n"}},
		},
	}
}
