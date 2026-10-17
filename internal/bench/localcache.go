package bench

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// StartLocalCache starts the bundled remote cache server (cmd/remotecache)
// with its data in dir and returns its address and a function that stops it.
// The server's log goes to dir/server.log.
func StartLocalCache(server, dir string) (string, func(), error) {
	if server == "" {
		return "", nil, fmt.Errorf("no -remote-cache-server (run hashbench with bazel run)")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", nil, err
	}
	logf, err := os.OpenFile(filepath.Join(dir, "server.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return "", nil, err
	}
	cmd := exec.Command(server, "--dir", filepath.Join(dir, "data"), "--listen", "127.0.0.1:0")
	cmd.Stderr = logf
	out, err := cmd.StdoutPipe()
	if err != nil {
		return "", nil, err
	}
	if err := cmd.Start(); err != nil {
		return "", nil, err
	}
	stop := func() {
		cmd.Process.Kill()
		cmd.Wait()
		logf.Close()
	}
	addr := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			if a, ok := strings.CutPrefix(sc.Text(), "listening on "); ok {
				addr <- a
			}
		}
	}()
	select {
	case a := <-addr:
		return a, stop, nil
	case <-time.After(10 * time.Second):
		stop()
		return "", nil, fmt.Errorf("%s did not start", server)
	}
}
