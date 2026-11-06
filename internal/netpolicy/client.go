// Package netpolicy 是 Feed、网页发现与媒体的唯一受控 HTTP 出口。
package netpolicy

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/idna"
)

const FeedLimit int64 = 10 << 20
const ImageLimit int64 = 8 << 20

var (
	ErrBlocked    = errors.New("网络目标不被允许")
	ErrInvalidURL = errors.New("网址无效")
	ErrTooLarge   = errors.New("响应超过大小限制")
	ErrEncoding   = errors.New("响应压缩格式不支持")
)

type Request struct {
	MaxBytes                   int64
	Timeout                    time.Duration
	ETag, LastModified, Accept string
}
type Response struct {
	URL    string
	Status int
	Header http.Header
	Body   []byte
}
type Fetcher interface {
	Fetch(context.Context, string, Request) (Response, error)
}
type Client struct {
	client *http.Client
	slots  chan struct{}
}
type resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}
type dialFunc func(context.Context, string, string) (net.Conn, error)

func New() *Client {
	return newClient(net.DefaultResolver, (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext)
}
func newClient(lookup resolver, dial dialFunc) *Client {
	transport := &http.Transport{
		Proxy: nil, DisableCompression: true, ForceAttemptHTTP2: false,
		MaxIdleConns: 8, MaxIdleConnsPerHost: 2, MaxConnsPerHost: 2, IdleConnTimeout: time.Minute,
		TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 15 * time.Second, MaxResponseHeaderBytes: 64 << 10,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, ErrInvalidURL
			}
			ips, err := resolve(ctx, lookup, host)
			if err != nil {
				return nil, err
			}
			var last error
			for _, ip := range ips {
				conn, err := dial(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
				last = err
			}
			return nil, last
		},
	}
	return &Client{slots: make(chan struct{}, 4), client: &http.Client{Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 5 {
			return ErrBlocked
		}
		if _, err := NormalizeURL(req.URL.String()); err != nil {
			return err
		}
		if _, err := resolve(req.Context(), lookup, req.URL.Hostname()); err != nil {
			return err
		}
		// 不允许跨站继承任何调用者的条件请求或敏感 Header。
		if len(via) > 0 && via[0].URL.Host != req.URL.Host {
			for _, key := range []string{"Authorization", "Cookie", "Proxy-Authorization", "If-None-Match", "If-Modified-Since", "Referer"} {
				req.Header.Del(key)
			}
		}
		return nil
	}}}
}
func (c *Client) CloseIdleConnections() { c.client.CloseIdleConnections() }

func NormalizeURL(raw string) (string, error) {
	if len(raw) > 8192 {
		return "", ErrInvalidURL
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || u.User != nil || u.Hostname() == "" || u.Opaque != "" {
		return "", ErrInvalidURL
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", ErrInvalidURL
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if strings.Contains(host, "%") || host == "" {
		return "", ErrInvalidURL
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if !publicIP(ip) {
			return "", ErrBlocked
		}
		host = ip.Unmap().String()
	} else {
		host, err = idna.Lookup.ToASCII(host)
		if err != nil || host == "" {
			return "", ErrInvalidURL
		}
		if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
			return "", ErrBlocked
		}
	}
	port := u.Port()
	if port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", ErrInvalidURL
		}
	}
	if u.Scheme == "http" && port == "80" || u.Scheme == "https" && port == "443" {
		port = ""
	}
	if port != "" {
		u.Host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		u.Host = "[" + host + "]"
	} else {
		u.Host = host
	}
	if u.Path == "" {
		u.Path = "/"
	}
	u.Fragment = ""
	return u.String(), nil
}

var fakeIPRange = netip.MustParsePrefix("198.18.0.0/15")

var reserved = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("192.88.99.0/24"), fakeIPRange,
	netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/96"), netip.MustParsePrefix("64:ff9b::/96"), netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"), netip.MustParsePrefix("2001::/23"), netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2002::/16"),
}

func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	if ip.Is6() && !netip.MustParsePrefix("2000::/3").Contains(ip) {
		return false
	}
	for _, prefix := range reserved {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}
func resolve(ctx context.Context, lookup resolver, host string) ([]netip.Addr, error) {
	if ip, err := netip.ParseAddr(host); err == nil {
		if !publicIP(ip) {
			return nil, ErrBlocked
		}
		return []netip.Addr{ip.Unmap()}, nil
	}
	ips, err := lookup.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	// 系统 DNS 可以同时返回可用的公网地址和私有地址。
	// 域名解析得到的 Fake-IP 交给本机代理处理。拨号使用筛出的固定 IP。
	allowed := make([]netip.Addr, 0, len(ips))
	for _, ip := range ips {
		if publicIP(ip) || fakeIPRange.Contains(ip.Unmap()) {
			allowed = append(allowed, ip.Unmap())
		}
	}
	if len(allowed) == 0 {
		return nil, ErrBlocked
	}
	return allowed, nil
}
func (c *Client) Fetch(ctx context.Context, raw string, options Request) (Response, error) {
	target, err := NormalizeURL(raw)
	if err != nil {
		return Response{}, err
	}
	limit := options.MaxBytes
	if limit <= 0 || limit > FeedLimit {
		limit = FeedLimit
	}
	timeout := options.Timeout
	if timeout <= 0 || timeout > 20*time.Second {
		timeout = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	case <-ctx.Done():
		return Response{}, ctx.Err()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return Response{}, ErrInvalidURL
	}
	req.Header.Set("User-Agent", "Fealdhord/0.1 RSS Reader")
	req.Header.Set("Accept-Encoding", "gzip")
	if options.Accept != "" {
		req.Header.Set("Accept", options.Accept)
	}
	if options.ETag != "" {
		req.Header.Set("If-None-Match", options.ETag)
	}
	if options.LastModified != "" {
		req.Header.Set("If-Modified-Since", options.LastModified)
	}
	response, err := c.client.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer response.Body.Close()
	result := Response{URL: response.Request.URL.String(), Status: response.StatusCode, Header: response.Header.Clone()}
	if response.StatusCode == http.StatusNotModified {
		return result, nil
	}
	if response.ContentLength > limit {
		return Response{}, ErrTooLarge
	}
	compressed := &budgetReader{reader: response.Body, remaining: limit}
	var body io.Reader = compressed
	switch strings.ToLower(strings.TrimSpace(response.Header.Get("Content-Encoding"))) {
	case "", "identity":
	case "gzip":
		reader, err := gzip.NewReader(compressed)
		if err != nil {
			return Response{}, err
		}
		defer reader.Close()
		body = reader
	default:
		return Response{}, ErrEncoding
	}
	result.Body, err = io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return Response{}, err
	}
	if int64(len(result.Body)) > limit {
		return Response{}, ErrTooLarge
	}
	return result, nil
}

type budgetReader struct {
	reader    io.Reader
	remaining int64
}

func (r *budgetReader) Read(p []byte) (int, error) {
	if int64(len(p)) > r.remaining+1 {
		p = p[:r.remaining+1]
	}
	n, err := r.reader.Read(p)
	r.remaining -= int64(n)
	if r.remaining < 0 {
		return n, ErrTooLarge
	}
	return n, err
}
