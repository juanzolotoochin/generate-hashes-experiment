// Package scenario defines the workspace mutations the benchmark measures.
package scenario

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed all:data
var dataFS embed.FS

// Op is one edit to the workspace.
type Op interface {
	Apply(root string) error
	String() string
}

// Append adds text to the end of a file.
type Append struct{ Path, Text string }

func (o Append) Apply(root string) error {
	f, err := os.OpenFile(filepath.Join(root, o.Path), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(o.Text); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
func (o Append) String() string { return "append to " + o.Path }

// Prepend adds text to the start of a file.
type Prepend struct{ Path, Text string }

func (o Prepend) Apply(root string) error {
	p := filepath.Join(root, o.Path)
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	return writeKeepMode(p, append([]byte(o.Text), b...))
}
func (o Prepend) String() string { return "prepend to " + o.Path }

// Replace substitutes Old with New in a file; Old must occur.
type Replace struct{ Path, Old, New string }

func (o Replace) Apply(root string) error {
	p := filepath.Join(root, o.Path)
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	s := string(b)
	if !strings.Contains(s, o.Old) {
		return fmt.Errorf("%s: %q not found", o.Path, o.Old)
	}
	return writeKeepMode(p, []byte(strings.ReplaceAll(s, o.Old, o.New)))
}
func (o Replace) String() string { return "edit " + o.Path }

// Write creates or overwrites a file.
type Write struct{ Path, Content string }

func (o Write) Apply(root string) error {
	p := filepath.Join(root, o.Path)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(o.Content), 0o644)
}
func (o Write) String() string { return "write " + o.Path }

// CopyData overwrites a workspace file with a pre-generated alternate version
// from the embedded data directory (real lockfiles produced by each ruleset's
// own tooling).
type CopyData struct{ Path, Data string }

func (o CopyData) Apply(root string) error {
	b, err := dataFS.ReadFile("data/" + o.Data)
	if err != nil {
		return err
	}
	return writeKeepMode(filepath.Join(root, o.Path), b)
}
func (o CopyData) String() string { return "replace " + o.Path + " with data/" + o.Data }

// Rename moves a file.
type Rename struct{ From, To string }

func (o Rename) Apply(root string) error {
	return os.Rename(filepath.Join(root, o.From), filepath.Join(root, o.To))
}
func (o Rename) String() string { return "rename " + o.From + " -> " + o.To }

// Chmod sets a file's mode.
type Chmod struct {
	Path string
	Mode os.FileMode
}

func (o Chmod) Apply(root string) error { return os.Chmod(filepath.Join(root, o.Path), o.Mode) }
func (o Chmod) String() string          { return fmt.Sprintf("chmod %o %s", o.Mode, o.Path) }

func writeKeepMode(p string, b []byte) error {
	st, err := os.Stat(p)
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, st.Mode().Perm())
}
