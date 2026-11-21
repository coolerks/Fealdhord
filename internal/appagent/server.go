package appagent

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coolerks/Fealdhord/internal/aiconfig"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ServerConfig struct {
	Enabled     bool     `json:"enabled"`
	External    bool     `json:"external"`
	Address     string   `json:"address"`
	Port        int      `json:"port"`
	TLS         bool     `json:"tls"`
	Certificate string   `json:"certificate"`
	PrivateKey  string   `json:"privateKey"`
	Tools       []string `json:"tools"`
}

type serverSettings struct {
	Config      ServerConfig
	TokenDigest string
	Clients     map[string]clientGrant
}

type clientGrant struct {
	Digest string
	Tools  []string
}

type MCPGrant struct {
	Name  string   `json:"name"`
	Tools []string `json:"tools"`
}

type ServerStatus struct {
	Config ServerConfig `json:"config"`
	URL    string       `json:"url"`
}

type sharedHandler struct{ current atomic.Pointer[http.Handler] }

func (h *sharedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	(*h.current.Load()).ServeHTTP(w, r)
}

type Platform struct {
	registry    *Registry
	dir         string
	mu          sync.Mutex
	state       serverSettings
	server      *http.Server
	endpoint    string
	clients     map[string]*clientConnection
	credentials aiconfig.CredentialStore
	clientEdit  sync.Mutex
}

func NewPlatform(registry *Registry, dir string) *Platform {
	a := &Platform{registry: registry, dir: dir, clients: make(map[string]*clientConnection), credentials: aiconfig.SystemCredentials{}}
	registry.auditDir = dir
	a.state.Config.Port = 8769
	if data, err := os.ReadFile(filepath.Join(dir, "mcp-server.json")); err == nil && len(data) < 64<<10 {
		_ = json.Unmarshal(data, &a.state)
		if a.state.Config.Enabled {
			if err := a.configure(a.state); err != nil {
				a.state.Config.Enabled = false
			}
		}
	}
	return a
}

func (a *Platform) Shutdown() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.server != nil {
		_ = a.server.Close()
		a.server = nil
	}
	for _, client := range a.clients {
		_ = client.session.Close()
	}
	clear(a.clients)
}

func (a *Platform) ToolList() []Tool                        { return a.registry.Tools() }
func (a *Platform) PendingActions() []PendingAction         { return a.registry.Pending() }
func (a *Platform) DecideAction(id string, allow bool) bool { return a.registry.Decide(id, allow) }
func (a *Platform) MCPServer() ServerStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return ServerStatus{Config: a.state.Config, URL: a.endpoint}
}

func (a *Platform) SaveMCPServer(config ServerConfig) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	next := a.state
	next.Config = config
	return a.configure(next)
}

func (a *Platform) RotateMCPToken() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	value := rand.Text() + rand.Text()
	digest := sha256.Sum256([]byte(value))
	next := a.state
	next.TokenDigest = hex.EncodeToString(digest[:])
	if err := a.configure(next); err != nil {
		return "", err
	}
	return value, nil
}

func atomicJSON(dir, name string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, "write-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, err = file.Write(data)
	if err = errors.Join(err, file.Close()); err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(dir, name))
}

func (a *Platform) configure(next serverSettings) error {
	config := next.Config
	if a.dir == "" || config.Port < 1024 || config.Port > 65535 {
		return errors.New("InvalidServerSettings")
	}
	for _, name := range config.Tools {
		if !slices.ContainsFunc(a.registry.tools, func(tool Tool) bool { return tool.Name == name }) {
			return errors.New("UnknownTool")
		}
	}
	var listener net.Listener
	var server *http.Server
	var endpoint string
	if config.Enabled {
		address := netip.MustParseAddr("127.0.0.1")
		if config.External {
			chosen, err := netip.ParseAddr(config.Address)
			if err != nil || chosen.IsUnspecified() || chosen.IsLoopback() || chosen.IsMulticast() || (!config.TLS && !chosen.IsPrivate()) {
				return errors.New("InvalidListenAddress")
			}
			address = chosen
		}
		if next.TokenDigest == "" && len(next.Clients) == 0 {
			return errors.New("CreateCredentialFirst")
		}
		var certificate tls.Certificate
		scheme := "http"
		if config.TLS {
			var err error
			certificate, err = tls.LoadX509KeyPair(config.Certificate, config.PrivateKey)
			if err != nil {
				return errors.New("InvalidTLSCertificate")
			}
			scheme = "https"
		}
		host := net.JoinHostPort(address.String(), strconv.Itoa(config.Port))
		endpoint = scheme + "://" + host + "/mcp"
		if a.server != nil && a.server.Addr == host && config.TLS == a.state.Config.TLS &&
			config.Certificate == a.state.Config.Certificate && config.PrivateKey == a.state.Config.PrivateKey {
			if atomicJSON(a.dir, "mcp-server.json", next) != nil {
				return errors.New("StorageError")
			}
			handler := a.handler(next, host, scheme)
			a.server.Handler.(*sharedHandler).current.Store(&handler)
			a.state = next
			return nil
		}
		var err error
		// A replacement on the same socket needs the old listener to release its port.
		// New settings are validated above before closing the old server.
		if a.server != nil && a.server.Addr == host {
			_ = a.server.Close()
			a.server = nil
			a.endpoint = ""
		}
		listener, err = net.Listen("tcp", host)
		if err != nil {
			return errors.New("ListenFailed")
		}
		if config.TLS {
			listener = tls.NewListener(listener, &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}})
		}
		handler := a.handler(next, host, scheme)
		shared := &sharedHandler{}
		shared.current.Store(&handler)
		server = &http.Server{Addr: host, Handler: shared, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 150 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	}
	if err := atomicJSON(a.dir, "mcp-server.json", next); err != nil {
		if listener != nil {
			_ = listener.Close()
		}
		return errors.New("StorageError")
	}
	if a.server != nil {
		_ = a.server.Close()
	}
	a.state, a.server, a.endpoint = next, server, endpoint
	if server != nil {
		go func() { _ = server.Serve(listener) }()
	}
	return nil
}

func (a *Platform) Grants() []MCPGrant {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]MCPGrant, 0, len(a.state.Clients))
	for name, grant := range a.state.Clients {
		out = append(out, MCPGrant{Name: name, Tools: slices.Clone(grant.Tools)})
	}
	slices.SortFunc(out, func(a, b MCPGrant) int { return strings.Compare(a.Name, b.Name) })
	return out
}

func (a *Platform) SaveGrant(grant MCPGrant, revoke bool) (string, error) {
	if !clientName.MatchString(grant.Name) {
		return "", errors.New("InvalidClientSettings")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	next := a.state
	next.Clients = make(map[string]clientGrant, len(a.state.Clients))
	for key, value := range a.state.Clients {
		next.Clients[key] = value
	}
	if revoke {
		delete(next.Clients, grant.Name)
		return "", a.configure(next)
	}
	if _, ok := next.Clients[grant.Name]; !ok && len(next.Clients) >= 32 {
		return "", errors.New("TooManyClients")
	}
	for _, name := range grant.Tools {
		if !slices.Contains(next.Config.Tools, name) {
			return "", errors.New("UnknownTool")
		}
	}
	value := rand.Text() + rand.Text()
	digest := sha256.Sum256([]byte(value))
	next.Clients[grant.Name] = clientGrant{Digest: hex.EncodeToString(digest[:]), Tools: slices.Clone(grant.Tools)}
	if err := a.configure(next); err != nil {
		return "", err
	}
	return value, nil
}

func (a *Platform) newMCPServer(config ServerConfig, tools []string, principal string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "Fealdhord", Version: "1"}, nil)
	for _, tool := range a.registry.tools {
		if !slices.Contains(config.Tools, tool.Name) || !slices.Contains(tools, tool.Name) {
			continue
		}
		server.AddTool(&mcp.Tool{Name: strings.ReplaceAll(tool.Name, ".", "_"), Description: tool.Description, InputSchema: tool.schema},
			func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				arguments := req.Params.Arguments
				if len(arguments) == 0 {
					arguments = json.RawMessage("{}")
				}
				result, err := a.registry.execute(ctx, principal, tool, arguments)
				reply := &mcp.CallToolResult{}
				if err != nil {
					reply.IsError = true
					result = err.Error()
				}
				reply.Content = []mcp.Content{&mcp.TextContent{Text: result}}
				return reply, nil
			})
	}
	return server
}

func (a *Platform) handler(config serverSettings, host, scheme string) http.Handler {
	servers := make(map[string]*mcp.Server, len(config.Clients)+1)
	if config.TokenDigest != "" {
		servers[config.TokenDigest] = a.newMCPServer(config.Config, config.Config.Tools, "MCP 默认客户端")
	}
	for name, grant := range config.Clients {
		servers[grant.Digest] = a.newMCPServer(config.Config, grant.Tools, "MCP "+name)
	}
	type authKey struct{}
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		server, _ := r.Context().Value(authKey{}).(*mcp.Server)
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	slots := make(chan struct{}, 8)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp" {
			http.NotFound(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		peer, err := netip.ParseAddrPort(r.RemoteAddr)
		if err != nil || !allowedPeer(config.Config, peer.Addr().Unmap()) {
			http.Error(w, "Remote access disabled", http.StatusForbidden)
			return
		}
		if r.Host != host || (origin != "" && origin != scheme+"://"+host) {
			http.Error(w, "Invalid host or origin", http.StatusForbidden)
			return
		}
		token, valid := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		digest := sha256.Sum256([]byte(token))
		var authorized *mcp.Server
		candidate := []byte(hex.EncodeToString(digest[:]))
		for expected, server := range servers {
			if subtle.ConstantTimeCompare(candidate, []byte(expected)) == 1 {
				authorized = server
			}
		}
		if !valid || authorized == nil {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
		handler.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), authKey{}, authorized)))
	})
}

func safeEndpoint(text string) (*url.URL, error) {
	endpoint, err := url.Parse(text)
	if err != nil || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.Fragment != "" ||
		(endpoint.Scheme != "https" && endpoint.Scheme != "http") {
		return nil, errors.New("InvalidEndpoint")
	}
	return endpoint, nil
}

func allowedPeer(config ServerConfig, peer netip.Addr) bool {
	return peer.IsLoopback() || (config.External && (config.TLS || peer.IsPrivate()))
}
