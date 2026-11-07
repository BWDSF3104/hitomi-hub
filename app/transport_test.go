package main

import (
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// R-07（assess-20261106-1056）合格条件テスト。既存テスト・fetch inject 口・gate は非触。
// http.DefaultTransport を直接変更しないこと（VT の http.DefaultClient とテスト inject の
// srv.Client() への波及回避）と、8 並行 burst での接続 reuse（確立回数 8）を assert する。
func TestPooledTransportSetsIdlePerHost16AndKeepsDefaultValues(t *testing.T) {
	def := http.DefaultTransport.(*http.Transport)
	tr := newPooledTransport()
	if tr == def {
		t.Fatal("newPooledTransport must not return http.DefaultTransport itself (VT and test inject share it)")
	}
	// http.DefaultTransport は MaxIdleConnsPerHost 0（実効 DefaultMaxIdleConnsPerHost=2）。
	// 直接変更不可（VT・テスト inject 波及）なので 0 のまま不动であることを assert。
	if def.MaxIdleConnsPerHost != 0 {
		t.Fatalf("http.DefaultTransport MaxIdleConnsPerHost changed: got %d, want 0 (effective 2 via DefaultMaxIdleConnsPerHost)", def.MaxIdleConnsPerHost)
	}
	if tr.MaxIdleConnsPerHost != pooledMaxIdleConnsPerHost {
		t.Fatalf("pooled transport MaxIdleConnsPerHost = %d, want %d", tr.MaxIdleConnsPerHost, pooledMaxIdleConnsPerHost)
	}
	if tr.IdleConnTimeout != def.IdleConnTimeout || tr.TLSHandshakeTimeout != def.TLSHandshakeTimeout ||
		tr.TLSClientConfig == nil || tr.ForceAttemptHTTP2 != def.ForceAttemptHTTP2 || tr.Proxy == nil {
		t.Fatalf("clone must keep default IdleConnTimeout/TLSHandshakeTimeout/ForceAttemptHTTP2/ProxyFromEnvironment")
	}
}

// 8 並行・round barrier で完結を位相同調させた burst（gate なし）。同一 helper transport では
// 確立回数は 8（全 request 確立後の round boundary で idle pool が 8 本同时に揃い、16 に収まる
// ため round 2 以降は reuse）。既存 default（idle 2）では同一 boundary で 6 本 close され、
// round 2 以降に再接続が発生して 8 を超える。handler sleep は TLS ハンドシェイク相当の注入
// 遅延ではなく確立回数計上の model（mock・外部 API なし）。
func TestPooledTransportReuses8ConnectionsUnder8ParallelBurst(t *testing.T) {
	var accepts atomic.Int64
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	counting := &countListener{Listener: ln, accepts: &accepts}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = w.Write([]byte("ok"))
	})
	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(counting)
	}()
	defer server.Close()

	burst := func(tr http.RoundTripper) int64 {
		accepts.Store(0)
		client := &http.Client{Transport: tr, Timeout: 5 * time.Second}
		base := "http://" + ln.Addr().String()
		start := make(chan struct{})
		var mu sync.Mutex
		cond := sync.NewCond(&mu)
		arrived := 0
		barrier := func() {
			mu.Lock()
			arrived++
			if arrived == 8 {
				cond.Broadcast()
			}
			for arrived < 8 {
				cond.Wait()
			}
			mu.Unlock()
		}
		var wg sync.WaitGroup
		for w := 0; w < 8; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				for r := 0; r < 5; r++ {
					resp, err := client.Get(base + "/x")
					if err != nil {
						t.Errorf("request: %v", err)
						return
					}
					// readAll+Close の順（hitomi fetch と同じ: body を読み切った直後に Close）
					_, _ = io.ReadAll(resp.Body)
					resp.Body.Close()
					barrier()
				}
			}()
		}
		close(start)
		wg.Wait()
		return accepts.Load()
	}

	got := burst(newPooledTransport())
	if got != 8 {
		t.Fatalf("pooled transport connection establishments = %d, want 8 (all requests reused)", got)
	}
	defGot := burst(http.DefaultTransport)
	if defGot <= 8 {
		t.Fatalf("default transport establishments = %d, want > 8 (idle 2 must close 6 at synchronized round boundary)", defGot)
	}
	t.Logf("burst 8並行×5round=40 request: pooled(16)=%d connections, default(2)=%d connections", got, defGot)
}

// viewer 構築口（実運用）が pooled transport を使うこと。テスト inject 口（c.fetch）とは経路が
// 別であることを fetch inject で確認する。
func TestViewerUsesPooledTransportAndFetchInjectPathUntouched(t *testing.T) {
	v := newViewer(nil, Config{}, t.TempDir())
	tr, ok := v.client.http.Transport.(*http.Transport)
	if !ok || tr.MaxIdleConnsPerHost != pooledMaxIdleConnsPerHost {
		t.Fatalf("viewer client transport = %v, want pooled transport with MaxIdleConnsPerHost %d", v.client.http.Transport, pooledMaxIdleConnsPerHost)
	}
	if v.client.fetch != nil {
		t.Fatal("fetch inject must default to nil (inject path untouched)")
	}
}

type countListener struct {
	net.Listener
	accepts *atomic.Int64
}

func (l *countListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err == nil {
		l.accepts.Add(1)
	}
	return conn, err
}
