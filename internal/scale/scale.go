// Package scale grows the fixture with generated packages for performance runs.
package scale

import (
	"fmt"
	"os"
	"path/filepath"
)

// Generate writes n service packages under scale/ in the workspace. Each
// package has Go, Python, C++, Rust, filegroup and custom-rule targets. They
// depend on the fixture's libraries and on the previous package, so the graph
// has both breadth and depth: an edit to a fixture library fans out to every
// generated package.
func Generate(workspace string, n int) error {
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("svc_%05d", i)
		dir := filepath.Join(workspace, "scale", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		goDeps := `"//go/greet"`
		pyDeps := `"//py/lib"`
		ccDeps := `"//cc/lib:hello"`
		rsDeps := `"//rust/lib"`
		if i > 0 {
			prev := fmt.Sprintf("//scale/svc_%05d", i-1)
			goDeps += fmt.Sprintf(`, "%s:go"`, prev)
			pyDeps += fmt.Sprintf(`, "%s:py"`, prev)
			ccDeps += fmt.Sprintf(`, "%s:cc"`, prev)
			rsDeps += fmt.Sprintf(`, "%s:rs"`, prev)
		}
		files := map[string]string{
			"BUILD.bazel": fmt.Sprintf(`load("@rules_cc//cc:cc_library.bzl", "cc_library")
load("@rules_go//go:def.bzl", "go_library", "go_test")
load("@rules_python//python:py_library.bzl", "py_library")
load("@rules_rs//rs:rust_library.bzl", "rust_library")
load("//tools:defs.bzl", "text_bundle")

go_library(
    name = "go",
    srcs = ["svc.go"],
    importpath = "example.com/hashbench/scale/%[1]s",
    visibility = ["//visibility:public"],
    deps = [%[2]s],
)

go_test(
    name = "go_test",
    srcs = ["svc_test.go"],
    embed = [":go"],
)

py_library(
    name = "py",
    srcs = ["svc.py"],
    visibility = ["//visibility:public"],
    deps = [%[3]s],
)

cc_library(
    name = "cc",
    srcs = ["svc.cc"],
    hdrs = ["svc.h"],
    visibility = ["//visibility:public"],
    deps = [%[4]s],
)

rust_library(
    name = "rs",
    srcs = ["svc.rs"],
    crate_name = "%[1]s",
    visibility = ["//visibility:public"],
    deps = [%[5]s],
)

filegroup(
    name = "srcs",
    srcs = glob(["svc.*"]),
)

text_bundle(
    name = "bundle",
    srcs = [":srcs"],
)
`, name, goDeps, pyDeps, ccDeps, rsDeps),
			"svc.go": fmt.Sprintf(`// Package %[1]s is a generated service.
package %[1]s

import "example.com/hashbench/go/greet"

// Hello greets from %[1]s.
func Hello() string { return greet.Greeting(%[1]q) }
`, name),
			"svc_test.go": fmt.Sprintf(`package %[1]s

import "testing"

func TestHello(t *testing.T) {
	if Hello() == "" {
		t.Fatal("empty greeting")
	}
}
`, name),
			"svc.py": fmt.Sprintf(`"""Generated service %[1]s."""

from py.lib import greet


def hello():
    return greet.greeting(%[1]q)
`, name),
			"svc.h":  fmt.Sprintf("#pragma once\n\n#include <string>\n\nstd::string Hello_%s();\n", name),
			"svc.cc": fmt.Sprintf("#include \"scale/%[1]s/svc.h\"\n\n#include \"cc/lib/hello.h\"\n\nstd::string Hello_%[1]s() { return Hello(%[1]q); }\n", name),
			"svc.rs": fmt.Sprintf("//! Generated service %[1]s.\n\npub fn hello() -> String {\n    hashbench_lib::counter(%[2]d)\n}\n", name, i),
		}
		for f, content := range files {
			if err := os.WriteFile(filepath.Join(dir, f), []byte(content), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}
