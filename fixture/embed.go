// Package fixture embeds the benchmark workspace.
package fixture

import "embed"

// FS holds the fixture workspace under Root.
//
//go:embed all:_workspace
var FS embed.FS

// Root is the directory inside FS that holds the workspace.
const Root = "_workspace"
