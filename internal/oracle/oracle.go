// Package oracle computes ground truth by building every target and
// fingerprinting what it depends on and what it produces.
package oracle

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/juanzolotoochin/generate-hashes-experiment/internal/tools"
	"github.com/juanzolotoochin/generate-hashes-experiment/internal/ws"
)

const outputGroup = "hashbench_digest"

// Oracle builds //... with the digest aspect applied.
type Oracle struct {
	Runner     *ws.Runner
	Bazel      string
	Workspace  string
	OutputBase string
	RepoDir    string // where the aspect repository is materialized

	src *sourceHasher
}

// Snapshot fingerprints every top-level target of one build two ways.
type Snapshot struct {
	// Inputs: changes when any action needed to build or test the target has
	// different inputs (the target must be rebuilt/retested). Primary truth.
	Inputs map[string]string
	// Outputs: changes when the target's output files, runfiles (contents or
	// modes) or test attributes differ. A subset of Inputs changes; the gap is
	// what Bazel's early cutoff saves.
	Outputs  map[string]string
	Failed   []string // targets that failed to build
	Duration time.Duration
}

// Install writes the aspect repository to o.RepoDir.
func (o *Oracle) Install() error {
	if err := os.RemoveAll(o.RepoDir); err != nil {
		return err
	}
	if err := ws.Materialize(repoFS, "repo", o.RepoDir); err != nil {
		return err
	}
	return installSelf(o.RepoDir)
}

func (o *Oracle) bazel(ctx context.Context, args ...string) ws.Result {
	return o.Runner.Run(ctx, o.Workspace, o.Bazel, append([]string{"--output_base=" + o.OutputBase}, args...)...)
}

// Build builds every target in the workspace and fingerprints them.
func (o *Oracle) Build(ctx context.Context, tag string) (Snapshot, error) {
	if o.src == nil {
		o.src = newSourceHasher()
	}
	start := time.Now()
	bep := filepath.Join(filepath.Dir(o.OutputBase), "bep-"+tag+".json")
	res := o.bazel(ctx, "build", "//...",
		"--keep_going",
		"--inject_repository=hashbench_oracle="+o.RepoDir,
		"--aspects=@hashbench_oracle//:digest.bzl%digest_aspect",
		"--output_groups="+outputGroup,
		"--build_event_json_file="+bep,
		"--build_event_json_file_path_conversion=false",
	)
	files, failed, err := parseBEP(bep)
	snap := Snapshot{Inputs: map[string]string{}, Outputs: map[string]string{}, Failed: failed}
	if err != nil {
		return snap, fmt.Errorf("parse BEP: %w (build: %v)", err, res.Err)
	}
	if res.Err != nil && len(files) == 0 {
		return snap, res.Err
	}

	aq := o.bazel(ctx, "aquery", "deps(//...)", "--output=jsonproto", "--include_commandline=false", "--keep_going")
	if aq.Err != nil && len(aq.Stdout) == 0 {
		return snap, fmt.Errorf("aquery: %w", aq.Err)
	}
	info := o.bazel(ctx, "info", "execution_root")
	if info.Err != nil {
		return snap, info.Err
	}
	fp, err := newInputFingerprinter(bytes.NewReader(aq.Stdout), strings.TrimSpace(string(info.Stdout)), o.OutputBase, o.src)
	if err != nil {
		return snap, err
	}

	for label, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			return snap, err
		}
		in, out, err := fingerprints(content, fp)
		if err != nil {
			return snap, fmt.Errorf("%s: %w", label, err)
		}
		snap.Inputs[label] = in
		snap.Outputs[label] = out
	}
	if dir := os.Getenv("HASHBENCH_ORACLE_DEBUG"); dir != "" {
		// Debugging aid: every file's input fingerprint, to diff snapshots.
		b, _ := json.MarshalIndent(fp.memo, "", " ")
		os.WriteFile(filepath.Join(dir, "fingerprints-"+tag+".json"), b, 0o644)
	}
	snap.Duration = time.Since(start)
	return snap, nil
}

// fingerprints computes a target's input and output fingerprints from its
// digest file. Both include the target's test attributes and the current mode
// of every listed file (read fresh: the digest action is keyed on contents
// only, so a mode-only change leaves the recorded modes stale). The input
// fingerprint uses each file's transitive input fingerprint; the output
// fingerprint uses its content hash.
func fingerprints(digestFile []byte, fp *inputFingerprinter) (string, string, error) {
	in, out := sha256.New(), sha256.New()
	var ins, outs []string
	for _, line := range strings.Split(string(digestFile), "\n") {
		switch {
		case strings.HasPrefix(line, "label "), line == "":
		case strings.HasPrefix(line, "attrs "):
			fmt.Fprintln(in, line)
			fmt.Fprintln(out, line)
		default:
			// "<sha256> <recorded mode> <path>"
			parts := strings.SplitN(line, " ", 3)
			if len(parts) != 3 {
				return "", "", fmt.Errorf("malformed digest line %q", line)
			}
			sum, path := parts[0], parts[2]
			sub, err := fp.Fingerprint(path)
			if err != nil {
				return "", "", err
			}
			mode := fp.Mode(path)
			ins = append(ins, path+" "+mode+" "+sub)
			outs = append(outs, path+" "+mode+" "+sum)
		}
	}
	sort.Strings(ins)
	sort.Strings(outs)
	for _, e := range ins {
		fmt.Fprintln(in, e)
	}
	for _, e := range outs {
		fmt.Fprintln(out, e)
	}
	return hex.EncodeToString(in.Sum(nil)), hex.EncodeToString(out.Sum(nil)), nil
}

// Shutdown stops the oracle's Bazel server.
func (o *Oracle) Shutdown(ctx context.Context) { o.bazel(ctx, "shutdown") }

type fileSetRef struct {
	ID string `json:"id"`
}

type bepEvent struct {
	ID struct {
		NamedSet        *fileSetRef `json:"namedSet"`
		TargetCompleted *struct {
			Label  string `json:"label"`
			Aspect string `json:"aspect"`
		} `json:"targetCompleted"`
	} `json:"id"`
	NamedSetOfFiles *struct {
		Files []struct {
			URI string `json:"uri"`
		} `json:"files"`
		FileSets []fileSetRef `json:"fileSets"`
	} `json:"namedSetOfFiles"`
	Completed *struct {
		Success     bool `json:"success"`
		OutputGroup []struct {
			Name     string       `json:"name"`
			FileSets []fileSetRef `json:"fileSets"`
		} `json:"outputGroup"`
	} `json:"completed"`
}

// parseBEP returns each successfully built target's digest file and the
// targets whose digest aspect failed.
func parseBEP(path string) (map[string]string, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	type set struct{ files, kids []string }
	sets := map[string]*set{}
	type target struct {
		label string
		ok    bool
		roots []string
	}
	var targets []target
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<28)
	for sc.Scan() {
		var e bepEvent
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			return nil, nil, err
		}
		switch {
		case e.ID.NamedSet != nil && e.NamedSetOfFiles != nil:
			s := &set{}
			for _, fl := range e.NamedSetOfFiles.Files {
				s.files = append(s.files, fl.URI)
			}
			for _, k := range e.NamedSetOfFiles.FileSets {
				s.kids = append(s.kids, k.ID)
			}
			sets[e.ID.NamedSet.ID] = s
		case e.ID.TargetCompleted != nil && e.ID.TargetCompleted.Aspect != "":
			t := target{label: tools.NormalizeLabel(e.ID.TargetCompleted.Label)}
			if e.Completed != nil {
				t.ok = e.Completed.Success
				for _, og := range e.Completed.OutputGroup {
					if og.Name == outputGroup {
						for _, r := range og.FileSets {
							t.roots = append(t.roots, r.ID)
						}
					}
				}
			}
			targets = append(targets, t)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, nil, err
	}

	var collect func(id string, seen map[string]bool, out *[]string)
	collect = func(id string, seen map[string]bool, out *[]string) {
		if seen[id] || sets[id] == nil {
			return
		}
		seen[id] = true
		*out = append(*out, sets[id].files...)
		for _, k := range sets[id].kids {
			collect(k, seen, out)
		}
	}
	files := map[string]string{}
	var failed []string
	for _, t := range targets {
		if !t.ok {
			failed = append(failed, t.label)
			continue
		}
		var uris []string
		for _, r := range t.roots {
			collect(r, map[string]bool{}, &uris)
		}
		for _, u := range uris {
			p, err := uriPath(u)
			if err != nil {
				return nil, nil, err
			}
			files[t.label] = p
		}
	}
	sort.Strings(failed)
	return files, failed, nil
}

func uriPath(u string) (string, error) {
	if !strings.HasPrefix(u, "file://") {
		return "", fmt.Errorf("unsupported BEP file URI %q", u)
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	return parsed.Path, nil
}
