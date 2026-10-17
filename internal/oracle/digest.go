package oracle

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DigestToolName is the name the harness binary is installed under in the
// oracle repository; main dispatches to RunDigest when invoked as it.
const DigestToolName = "digest"

// RunDigest implements the aspect's action: digest <out> <label> <attrs> <file>...
// It writes "label", "attrs" and then one sorted "<sha256> <mode> <path>" line
// per regular file reachable from the given files (directories are expanded).
func RunDigest(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: digest OUT LABEL ATTRS FILE...")
	}
	out, label, attrs := args[0], args[1], args[2]
	seen := map[string]bool{}
	var lines []string
	var visit func(p string) error
	visit = func(p string) error {
		st, err := os.Stat(p)
		if err != nil {
			// A symlink whose target isn't an input (e.g. a Python venv's
			// interpreter link) is identified by its target.
			target, lerr := os.Readlink(p)
			if lerr != nil {
				return err
			}
			if !seen[p] {
				seen[p] = true
				sum := sha256.Sum256([]byte(target))
				lines = append(lines, fmt.Sprintf("%s 777 %s", hex.EncodeToString(sum[:]), p))
			}
			return nil
		}
		if st.IsDir() {
			entries, err := os.ReadDir(p)
			if err != nil {
				return err
			}
			for _, e := range entries {
				if err := visit(filepath.Join(p, e.Name())); err != nil {
					return err
				}
			}
			return nil
		}
		if seen[p] {
			return nil
		}
		seen[p] = true
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			return err
		}
		lines = append(lines, fmt.Sprintf("%s %o %s", hex.EncodeToString(h.Sum(nil)), st.Mode().Perm(), p))
		return nil
	}
	for _, p := range args[3:] {
		if err := visit(p); err != nil {
			return err
		}
	}
	sort.Strings(lines)
	var b strings.Builder
	fmt.Fprintf(&b, "label %s\nattrs %s\n", label, attrs)
	for _, l := range lines {
		b.WriteString(l + "\n")
	}
	return os.WriteFile(out, []byte(b.String()), 0o644)
}

// installSelf copies the running binary into the oracle repository as the
// digest tool, so the aspect needs nothing from the host.
func installSelf(repoDir string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	src, err := os.Open(self)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(filepath.Join(repoDir, DigestToolName), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	return dst.Close()
}
