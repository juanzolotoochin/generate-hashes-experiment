package oracle

import "embed"

// repoFS holds the injected Bazel repository that defines the digest aspect.
//
//go:embed all:repo
var repoFS embed.FS
