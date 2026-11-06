package netpolicy

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
)

type fixedResolver struct{ ips []netip.Addr }

func (r fixedResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return r.ips, nil
}
func localClient(t *testing.T, handler http.HandlerFunc) (*Client, *atomic.Int32) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	calls := &atomic.Int32{}
	c := newClient(fixedResolver{[]netip.Addr{netip.MustParseAddr("fd27:712::1"), netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("127.0.0.1")}}, func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "8.8.8.8:80" {
			t.Errorf("未连接已校验 IP：%s", address)
		}
		calls.Add(1)
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	})
	t.Cleanup(c.CloseIdleConnections)
	return c, calls
}
func TestBlockedTargets(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "http://user:pass@example.com/", "http://127.0.0.1/", "http://[::ffff:127.0.0.1]/", "http://10.0.0.1/", "http://169.254.169.254/", "http://100.64.0.1/", "http://192.0.2.1/", "http://198.18.0.42/", "http://[2001:db8::1]/", "http://[64:ff9b::7f00:1]/", "http://local.local/", "http://localhost./", "http://example.com:0/"} {
		if _, err := NormalizeURL(raw); err == nil {
			t.Errorf("危险目标未被拒绝：%s", raw)
		}
	}
	canonical, err := NormalizeURL("HTTPS://Example.COM:443/feed?token=a#part")
	if err != nil || canonical != "https://example.com/feed?token=a" {
		t.Fatalf("规范化失败 %q %v", canonical, err)
	}
}
func TestMixedDNSUsesOnlyPinnedPublicIP(t *testing.T) {
	c, calls := localClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "feed.example" {
			t.Errorf("Host 丢失：%s", r.Host)
		}
		io.WriteString(w, "ok")
	})
	response, err := c.Fetch(context.Background(), "http://feed.example/", Request{})
	if err != nil || string(response.Body) != "ok" || calls.Load() != 1 {
		t.Fatalf("安全抓取失败 %+v %v", response, err)
	}
	c = newClient(fixedResolver{[]netip.Addr{netip.MustParseAddr("fd27:712::1"), netip.MustParseAddr("127.0.0.1")}}, func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("仅含私网的 DNS 不应拨号")
		return nil, nil
	})
	if _, err = c.Fetch(context.Background(), "http://feed.example/", Request{}); !errors.Is(err, ErrBlocked) {
		t.Fatalf("仅含私网的 DNS 未拒绝：%v", err)
	}
}
func TestFakeIPDNSAllowsSubscriptionAndRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host == "feed.example" {
			http.Redirect(w, r, "http://other.example/feed", http.StatusFound)
			return
		}
		if r.Host != "other.example" {
			t.Errorf("Host 丢失：%s", r.Host)
		}
		io.WriteString(w, "feed")
	}))
	defer server.Close()

	lookup := fixedResolver{[]netip.Addr{netip.MustParseAddr("198.18.0.42")}}
	client := newClient(lookup, func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "198.18.0.42:80" {
			t.Errorf("未连接 DNS 返回的 Fake-IP：%s", address)
		}
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	})
	defer client.CloseIdleConnections()

	response, err := client.Fetch(context.Background(), "http://feed.example/", Request{})
	if err != nil || string(response.Body) != "feed" {
		t.Fatalf("Fake-IP 抓取失败：%q %v", response.Body, err)
	}
}

func TestRedirectsAndCrossOriginValidators(t *testing.T) {
	c, _ := localClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/private":
			http.Redirect(w, r, "http://127.0.0.1/secret", http.StatusFound)
		case "/start":
			http.Redirect(w, r, "http://other.example/end", http.StatusFound)
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		default:
			if r.Header.Get("If-None-Match") != "" {
				t.Error("跨域泄漏条件请求")
			}
			io.WriteString(w, "done")
		}
	})
	for _, path := range []string{"/private", "/loop"} {
		if _, err := c.Fetch(context.Background(), "http://feed.example"+path, Request{}); !errors.Is(err, ErrBlocked) {
			t.Fatalf("跳转未拒绝：%s %v", path, err)
		}
	}
	response, err := c.Fetch(context.Background(), "http://feed.example/start", Request{ETag: "secret-validator"})
	if err != nil || string(response.Body) != "done" {
		t.Fatalf("跨域正常跳转失败：%v", err)
	}
}
func TestBoundedDecompressionAndConditional304(t *testing.T) {
	var compressed bytes.Buffer
	zip := gzip.NewWriter(&compressed)
	zip.Write([]byte(strings.Repeat("a", 2048)))
	zip.Close()
	c, _ := localClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/304" {
			if r.Header.Get("If-None-Match") != "v1" {
				t.Error("未发送 validator")
			}
			w.WriteHeader(304)
			return
		}
		if r.URL.Path == "/gzip" {
			w.Header().Set("Content-Encoding", "gzip")
			w.Write(compressed.Bytes())
			return
		}
		w.Write([]byte(strings.Repeat("b", 2048)))
	})
	for _, path := range []string{"/gzip", "/plain"} {
		if _, err := c.Fetch(context.Background(), "http://feed.example"+path, Request{MaxBytes: 128}); !errors.Is(err, ErrTooLarge) {
			t.Fatalf("未限制解压或原始字节：%s %v", path, err)
		}
	}
	response, err := c.Fetch(context.Background(), "http://feed.example/304", Request{ETag: "v1"})
	if err != nil || response.Status != 304 || len(response.Body) != 0 {
		t.Fatalf("304 不正确：%+v %v", response, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Fetch(ctx, "http://feed.example/", Request{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("取消未贯穿：%v", err)
	}
}

type resolverFunc func(context.Context, string, string) ([]netip.Addr, error)

func (f resolverFunc) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	return f(ctx, network, host)
}
func TestRebindingBetweenRedirectCheckAndDial(t *testing.T) {
	var requests, lookups, dials atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Redirect(w, r, "http://rebound.example/end", 302)
	}))
	defer server.Close()
	c := newClient(resolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		if lookups.Add(1) >= 3 {
			return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
		}
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}), func(ctx context.Context, network, address string) (net.Conn, error) {
		dials.Add(1)
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	})
	defer c.CloseIdleConnections()
	if _, err := c.Fetch(context.Background(), "http://feed.example/start", Request{}); !errors.Is(err, ErrBlocked) {
		t.Fatalf("重绑定未被拦截：%v", err)
	}
	if requests.Load() != 1 || dials.Load() != 1 {
		t.Fatal("连接了重绑定目标")
	}
}

func TestPublicDialFailureDoesNotFallBackToPrivateDNS(t *testing.T) {
	dialFailure := errors.New("公网连接失败")
	c := newClient(fixedResolver{[]netip.Addr{
		netip.MustParseAddr("8.8.8.8"),
		netip.MustParseAddr("fd27:712::1"),
		netip.MustParseAddr("::ffff:127.0.0.1"),
	}}, func(_ context.Context, _, address string) (net.Conn, error) {
		if address != "8.8.8.8:80" {
			t.Errorf("尝试了未经允许的地址：%s", address)
		}
		return nil, dialFailure
	})
	defer c.CloseIdleConnections()
	if _, err := c.Fetch(context.Background(), "http://feed.example/", Request{}); !errors.Is(err, dialFailure) {
		t.Fatalf("应保留公网连接错误：%v", err)
	}
}
