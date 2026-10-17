// Package netmeter counts the bytes a Bazel server downloads. Each tool gets
// its own Meter: an HTTP CONNECT proxy for internet downloads (passed to Bazel
// as HTTPS_PROXY) and, optionally, a Unix socket forwarding to the remote cache
// (passed as --remote_proxy=unix:...). Both see TLS traffic only, so the counts
// are bytes on the wire.
package netmeter

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
)

// Counts is the number of bytes received, by channel.
type Counts struct {
	Internet int64 `json:"internet"`
	Remote   int64 `json:"remote"`
	// Sent is the bytes sent on all channels (uploads), not part of Total.
	Sent int64 `json:"sent,omitempty"`
}

// Total is the sum of all channels.
func (c Counts) Total() int64 { return c.Internet + c.Remote }

// Sub returns c - o.
func (c Counts) Sub(o Counts) Counts {
	return Counts{Internet: c.Internet - o.Internet, Remote: c.Remote - o.Remote, Sent: c.Sent - o.Sent}
}

// Meter is a running pair of forwarders.
type Meter struct {
	internet, remote, sent atomic.Int64
	proxy                  net.Listener
	sock                   net.Listener
	sockPath               string
}

// Start starts the CONNECT proxy and, if remoteAddr (host:port) is set, a
// Unix socket at sockPath that forwards to it.
func Start(remoteAddr, sockPath string) (*Meter, error) {
	m := &Meter{}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	m.proxy = ln
	m.serve(ln, m.handleConnect)
	if remoteAddr != "" {
		os.Remove(sockPath)
		if err := os.MkdirAll(filepath.Dir(sockPath), 0o700); err != nil {
			ln.Close()
			return nil, err
		}
		sl, err := net.Listen("unix", sockPath)
		if err != nil {
			ln.Close()
			return nil, err
		}
		m.sock, m.sockPath = sl, sockPath
		m.serve(sl, func(c net.Conn) {
			up, err := net.Dial("tcp", remoteAddr)
			if err != nil {
				c.Close()
				return
			}
			m.pipe(c, up, &m.remote)
		})
	}
	return m, nil
}

// Env returns the proxy variables for a Bazel client (and its server).
func (m *Meter) Env() []string {
	u := "http://" + m.proxy.Addr().String()
	return []string{"HTTPS_PROXY=" + u, "https_proxy=" + u, "HTTP_PROXY=" + u, "http_proxy=" + u}
}

// RemoteProxy returns the --remote_proxy value, or "" without a remote.
func (m *Meter) RemoteProxy() string {
	if m.sock == nil {
		return ""
	}
	return "unix:" + m.sockPath
}

// Counts returns the bytes received so far.
func (m *Meter) Counts() Counts {
	return Counts{Internet: m.internet.Load(), Remote: m.remote.Load(), Sent: m.sent.Load()}
}

// Close stops accepting connections.
func (m *Meter) Close() {
	m.proxy.Close()
	if m.sock != nil {
		m.sock.Close()
		os.Remove(m.sockPath)
	}
}

func (m *Meter) serve(ln net.Listener, handle func(net.Conn)) {
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go handle(c)
		}
	}()
}

// handleConnect serves one CONNECT request (Bazel's downloader only fetches
// over HTTPS through a proxy) and then relays raw bytes.
func (m *Meter) handleConnect(c net.Conn) {
	br := bufio.NewReader(c)
	req, err := http.ReadRequest(br)
	if err != nil || req.Method != http.MethodConnect {
		io.WriteString(c, "HTTP/1.1 405 Method Not Allowed\r\nConnection: close\r\n\r\n")
		c.Close()
		return
	}
	up, err := net.Dial("tcp", req.Host)
	if err != nil {
		io.WriteString(c, "HTTP/1.1 502 Bad Gateway\r\nConnection: close\r\n\r\n")
		c.Close()
		return
	}
	io.WriteString(c, "HTTP/1.1 200 Connection Established\r\n\r\n")
	// Anything the client sent after the request line is already buffered.
	if n := br.Buffered(); n > 0 {
		b, _ := br.Peek(n)
		up.Write(b)
	}
	m.pipe(c, up, &m.internet)
}

// pipe relays both directions and counts upstream -> client bytes.
func (m *Meter) pipe(client, up net.Conn, rx *atomic.Int64) {
	done := make(chan struct{}, 2)
	go func() {
		io.Copy(up, countingReader{client, &m.sent})
		if tc, ok := up.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
		done <- struct{}{}
	}()
	go func() {
		io.Copy(client, countingReader{up, rx})
		if uc, ok := client.(interface{ CloseWrite() error }); ok {
			uc.CloseWrite()
		}
		done <- struct{}{}
	}()
	<-done
	<-done
	client.Close()
	up.Close()
}

type countingReader struct {
	r io.Reader
	n *atomic.Int64
}

func (c countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n.Add(int64(n))
	return n, err
}
