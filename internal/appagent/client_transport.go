package appagent

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net"
	"net/http"
	"net/netip"
	"os/exec"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpHTTPTransport struct {
	*http.Transport
	headers   map[string]string
	transient atomic.Bool
}

func (t *mcpHTTPTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	request = request.Clone(request.Context())
	for key, value := range t.headers {
		request.Header.Set(key, value)
	}
	response, err := t.Transport.RoundTrip(request)
	if err == nil && request.Method == http.MethodPost && (response.StatusCode >= 500 ||
		response.StatusCode == http.StatusRequestTimeout || response.StatusCode == http.StatusTooManyRequests) {
		t.transient.Store(true)
	}
	return response, err
}

func newMCPHTTPTransport(config ClientConfig, secrets clientSecrets) (*mcp.StreamableClientTransport, *mcpHTTPTransport, error) {
	endpoint, err := safeEndpoint(config.Endpoint)
	if err != nil {
		return nil, nil, err
	}
	if config.SkipTLSVerify && endpoint.Scheme != "https" {
		return nil, nil, errClientConfig
	}
	if endpoint.Scheme == "http" && (secrets.Bearer != "" || secrets.Headers["Authorization"] != "" ||
		secrets.Headers["Cookie"] != "" || secrets.Headers["X-Api-Key"] != "") && endpoint.Hostname() != "localhost" {
		ip, err := netip.ParseAddr(endpoint.Hostname())
		if err != nil || (!ip.IsPrivate() && !ip.IsLoopback()) {
			return nil, nil, errors.New("HTTPSRequiredForCredential")
		}
	}
	proxy, err := clientProxy(config, secrets.ProxyPassword)
	if err != nil {
		return nil, nil, err
	}
	timeout := time.Duration(config.ConnectionTimeout) * time.Second
	base := &http.Transport{
		DialContext:         (&net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout: timeout, IdleConnTimeout: 90 * time.Second, ForceAttemptHTTP2: true,
		// The user can opt out for this connection only.
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: config.SkipTLSVerify},
	}
	if proxy != nil {
		base.Proxy = http.ProxyURL(proxy)
	}
	headers := make(map[string]string, len(secrets.Headers)+1)
	maps.Copy(headers, secrets.Headers)
	if _, explicit := headers["Authorization"]; !explicit && secrets.Bearer != "" {
		headers["Authorization"] = "Bearer " + secrets.Bearer
	}
	httpTransport := &mcpHTTPTransport{Transport: base, headers: headers}
	retries := *config.ConnectionRetries
	if retries == 0 {
		retries = -1 // The MCP SDK uses zero for its own default.
	}
	return &mcp.StreamableClientTransport{
		Endpoint: endpoint.String(), MaxRetries: retries,
		HTTPClient: &http.Client{
			Transport:     httpTransport,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, httpTransport, nil
}

func openMCPClient(ctx context.Context, config ClientConfig, secrets clientSecrets, log *clientLogBuffer) (*clientConnection, error) {
	if log != nil {
		if config.Command == "" {
			return nil, errClientConfig
		}
		cmd := exec.Command(config.Command, config.Args...)
		cmd.Dir = config.WorkingDirectory
		cmd.Stderr = log
		cmd.Env = cmd.Environ()
		for name, value := range secrets.Environment {
			cmd.Env = append(cmd.Env, name+"="+value)
		}
		opts := &mcp.ClientOptions{LoggingMessageHandler: func(_ context.Context, request *mcp.LoggingMessageRequest) {
			if request.Params != nil {
				value, ok := request.Params.Data.(string)
				if !ok {
					data, _ := json.Marshal(request.Params.Data)
					value = string(data)
				}
				log.add("mcp", safeMCPLogLevel(string(request.Params.Level)), value)
			}
		}}
		client := mcp.NewClient(&mcp.Implementation{Name: "Fealdhord", Version: "1"}, opts)
		attempt, cancel := toolDeadline(ctx, config.StartupTimeout)
		defer cancel()
		session, err := client.Connect(attempt, &mcp.CommandTransport{Command: cmd, TerminateDuration: time.Second}, nil)
		if err != nil {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			return nil, connectionError(attempt)
		}
		return discoverMCPClient(attempt, session, config, log)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "Fealdhord", Version: "1"}, nil)
	for retry := 0; ; retry++ {
		attempt, cancel := toolDeadline(ctx, config.ConnectionTimeout)
		transport, httpTransport, err := newMCPHTTPTransport(config, secrets)
		if err != nil {
			cancel()
			return nil, err
		}
		session, err := client.Connect(attempt, transport, nil)
		if err == nil {
			conn, discoveryErr := discoverMCPClient(attempt, session, config, nil)
			cancel()
			return conn, discoveryErr
		}
		failure := connectionError(attempt)
		cancel()
		httpTransport.CloseIdleConnections()
		var network *net.OpError
		var certificate *tls.CertificateVerificationError
		transient := httpTransport.transient.Load() || errors.As(err, &network) ||
			errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || failure.Error() == "MCPConnectionTimeout"
		if !transient || errors.As(err, &certificate) || retry >= *config.ConnectionRetries || ctx.Err() != nil {
			return nil, failure
		}
		timer := time.NewTimer(time.Duration(min(1<<retry, 8)) * 250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, connectionError(ctx)
		case <-timer.C:
		}
	}
}

func connectionError(ctx context.Context) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return errors.New("MCPConnectionTimeout")
	}
	if ctx.Err() != nil {
		return errors.New("Cancelled")
	}
	return errors.New("MCPConnectionFailed")
}

func safeMCPLogLevel(level string) string {
	switch level {
	case "debug", "info", "notice", "warning", "error", "critical", "alert", "emergency":
		return level
	}
	return ""
}

const maxMCPTools = 4096
const maxMCPToolPages = 1024
const maxMCPToolCatalogBytes = 16 << 20

func discoverMCPClient(ctx context.Context, session *mcp.ClientSession, config ClientConfig, log *clientLogBuffer) (*clientConnection, error) {
	conn := &clientConnection{session: session, target: config, status: ClientStatus{
		ID: config.identity(), Name: config.Name, Connected: true, Tools: []string{}, Resources: []string{}, Prompts: []string{},
	}}
	initialized := session.InitializeResult()
	capabilities := initialized.Capabilities
	if capabilities == nil {
		return conn, nil
	}
	if log != nil && capabilities.Logging != nil && initialized.ProtocolVersion < "2026-07-28" {
		logContext, cancel := context.WithTimeout(ctx, time.Second)
		err := session.SetLoggingLevel(logContext, &mcp.SetLoggingLevelParams{Level: mcp.LoggingLevel(config.LogLevel)})
		cancel()
		if err != nil {
			log.add("client", "warning", "服务未接受设置日志等级。")
		}
	}
	if capabilities.Tools != nil {
		if err := conn.discoverTools(ctx); err != nil {
			_ = session.Close()
			return nil, err
		}
	}
	if capabilities.Resources != nil {
		for item, err := range session.Resources(ctx, nil) {
			if err != nil || len(conn.status.Resources) >= 64 {
				break
			}
			conn.status.Resources = append(conn.status.Resources, item.URI)
		}
	}
	if capabilities.Prompts != nil {
		for item, err := range session.Prompts(ctx, nil) {
			if err != nil || len(conn.status.Prompts) >= 32 {
				break
			}
			conn.status.Prompts = append(conn.status.Prompts, item.Name)
		}
	}
	if ctx.Err() != nil {
		_ = session.Close()
		return nil, connectionError(ctx)
	}
	return conn, nil
}

func (c *clientConnection) discoverTools(ctx context.Context) error {
	cursors := make(map[string]bool)
	names := make(map[string]bool)
	cursor, size := "", 0
	for range maxMCPToolPages {
		page, err := c.session.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			if ctx.Err() != nil {
				return connectionError(ctx)
			}
			return errors.New("MCPDiscoveryFailed")
		}
		if len(c.tools)+len(page.Tools) > maxMCPTools {
			return errors.New("MCPToolCatalogLimit")
		}
		for _, tool := range page.Tools {
			if tool == nil || tool.Name == "" || names[tool.Name] {
				return errors.New("MCPInvalidToolCatalog")
			}
			raw, err := json.Marshal(tool)
			if err != nil {
				return errors.New("MCPInvalidToolCatalog")
			}
			size += len(raw)
			if size > maxMCPToolCatalogBytes {
				return errors.New("MCPToolCatalogLimit")
			}
			names[tool.Name] = true
			c.tools = append(c.tools, tool)
			c.status.Tools = append(c.status.Tools, tool.Name)
		}
		cursor = page.NextCursor
		if cursor == "" {
			return nil
		}
		if cursors[cursor] {
			return errors.New("MCPInvalidToolCatalog")
		}
		cursors[cursor] = true
	}
	return errors.New("MCPToolCatalogLimit")
}
