package appagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/coolerks/Fealdhord/internal/agentrun"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/zalando/go-keyring"
)

const maxMCPClientSettings = 512 << 10

var clientName = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)

type ClientConfig struct {
	Name     string   `json:"name"`
	Endpoint string   `json:"endpoint"`
	Command  string   `json:"command"`
	Args     []string `json:"args"`
	Bearer   string   `json:"bearer,omitempty"`
}
type ClientStatus struct {
	Name      string       `json:"name"`
	Tools     []string     `json:"tools"`
	Resources []string     `json:"resources"`
	Prompts   []string     `json:"prompts"`
	Connected bool         `json:"connected"`
	Config    ClientConfig `json:"config"`
}
type bearerTransport struct{ token string }

func (t bearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	request = request.Clone(request.Context())
	if t.token != "" {
		request.Header.Set("Authorization", "Bearer "+t.token)
	}
	return http.DefaultTransport.RoundTrip(request)
}

type clientConnection struct {
	session *mcp.ClientSession
	tools   []*mcp.Tool
	status  ClientStatus
}

func (a *Platform) ConnectMCP(ctx context.Context, config ClientConfig) (ClientStatus, error) {
	a.clientEdit.Lock()
	defer a.clientEdit.Unlock()
	return a.connectMCP(ctx, config)
}

func (a *Platform) connectMCP(ctx context.Context, config ClientConfig) (ClientStatus, error) {
	if !clientName.MatchString(config.Name) || len(config.Bearer) > 4096 || strings.ContainsAny(config.Bearer, "\r\n") ||
		(config.Command == "") == (config.Endpoint == "") {
		return ClientStatus{}, errors.New("InvalidClientSettings")
	}
	values, err := a.savedClients()
	if err != nil {
		return ClientStatus{}, err
	}
	previous, exists := values[config.Name]
	if !exists && len(values) >= 32 {
		return ClientStatus{}, errors.New("TooManyClients")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var transport mcp.Transport
	bearer := config.Bearer
	if bearer == "" && config.Endpoint != "" {
		if exists && previous.Endpoint == config.Endpoint {
			bearer, _ = a.credentials.Get(clientCredentialRef(config))
		}
	}
	if config.Command != "" {
		transport = &mcp.CommandTransport{Command: exec.Command(config.Command, config.Args...)}
	} else {
		endpoint, err := safeEndpoint(config.Endpoint)
		if err != nil {
			return ClientStatus{}, err
		}
		if endpoint.Scheme == "http" && bearer != "" && endpoint.Hostname() != "localhost" {
			ip, err := netip.ParseAddr(endpoint.Hostname())
			if err != nil || (!ip.IsPrivate() && !ip.IsLoopback()) {
				return ClientStatus{}, errors.New("HTTPSRequiredForCredential")
			}
		}
		transport = &mcp.StreamableClientTransport{Endpoint: endpoint.String(), HTTPClient: &http.Client{
			Transport:     bearerTransport{token: bearer},
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		}, MaxRetries: -1}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "Fealdhord", Version: "1"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return ClientStatus{}, errors.New("MCPConnectionFailed")
	}
	status := ClientStatus{Name: config.Name, Connected: true, Tools: []string{}, Resources: []string{}, Prompts: []string{}}
	conn := &clientConnection{session: session, status: status}
	if capabilities := session.InitializeResult().Capabilities; capabilities != nil {
		if capabilities.Tools != nil {
			for tool, err := range session.Tools(ctx, nil) {
				if err != nil {
					_ = session.Close()
					return ClientStatus{}, errors.New("MCPDiscoveryFailed")
				}
				if len(conn.tools) >= 32 {
					break
				}
				conn.tools = append(conn.tools, tool)
				conn.status.Tools = append(conn.status.Tools, tool.Name)
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
	}
	if err := a.persistClient(config, values); err != nil {
		_ = session.Close()
		return ClientStatus{}, err
	}
	config.Bearer = ""
	conn.status.Config = config
	a.mu.Lock()
	defer a.mu.Unlock()
	if previous := a.clients[config.Name]; previous != nil {
		_ = previous.session.Close()
	}
	a.clients[config.Name] = conn
	go func() {
		_ = session.Wait()
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.clients[config.Name] == conn {
			delete(a.clients, config.Name)
		}
	}()
	return conn.status, nil
}

func (a *Platform) MCPClients() ([]ClientStatus, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	settings, err := a.savedClients()
	if err != nil {
		return nil, err
	}
	values := make([]ClientStatus, 0, len(a.clients)+len(settings))
	for name, config := range settings {
		if a.clients[name] == nil {
			values = append(values, ClientStatus{Name: name, Config: config})
		}
	}
	for _, conn := range a.clients {
		values = append(values, conn.status)
	}
	slices.SortFunc(values, func(a, b ClientStatus) int { return strings.Compare(a.Name, b.Name) })
	return values, nil
}

func (a *Platform) savedClients() (map[string]ClientConfig, error) {
	failure := errors.New("StorageError")
	if a.dir == "" {
		return nil, failure
	}
	values := make(map[string]ClientConfig)
	path := filepath.Join(a.dir, "mcp-clients.json")
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return values, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxMCPClientSettings {
		return nil, failure
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, failure
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxMCPClientSettings+1))
	if err != nil || len(data) > maxMCPClientSettings || json.Unmarshal(data, &values) != nil || values == nil {
		return nil, failure
	}
	return values, nil
}

func (a *Platform) persistClient(config ClientConfig, values map[string]ClientConfig) error {
	previous, exists := values[config.Name]
	ref := clientCredentialRef(config)
	bearer := config.Bearer
	config.Bearer = ""
	values[config.Name] = config
	data, err := json.Marshal(values)
	if err != nil || len(data) > maxMCPClientSettings {
		return errors.New("ClientSettingsTooLarge")
	}
	var previousBearer string
	if bearer != "" {
		if exists && previous.Endpoint == config.Endpoint {
			previousBearer, err = a.credentials.Get(ref)
			if err != nil && !errors.Is(err, keyring.ErrNotFound) {
				return errors.New("CredentialError")
			}
		}
		if err := a.credentials.Set(ref, bearer); err != nil {
			return errors.New("CredentialError")
		}
	}
	if err := atomicFile(a.dir, "mcp-clients.json", data); err != nil {
		if bearer != "" {
			if previousBearer == "" {
				_ = a.credentials.Delete(ref)
			} else if a.credentials.Set(ref, previousBearer) != nil {
				return errors.New("CredentialRestoreError")
			}
		}
		return errors.New("StorageError")
	}
	if exists && previous.Endpoint != config.Endpoint {
		_ = a.credentials.Delete(clientCredentialRef(previous))
	}
	return nil
}

func clientCredentialRef(config ClientConfig) string {
	digest := sha256.Sum256([]byte(config.Endpoint))
	return "mcp-client:" + config.Name + ":" + hex.EncodeToString(digest[:])
}

func (a *Platform) ForgetMCP(name string) error {
	if !clientName.MatchString(name) {
		return errors.New("InvalidClientSettings")
	}
	a.clientEdit.Lock()
	defer a.clientEdit.Unlock()
	values, err := a.savedClients()
	if err != nil {
		return err
	}
	previous, exists := values[name]
	delete(values, name)
	if err := atomicJSON(a.dir, "mcp-clients.json", values); err != nil {
		return errors.New("StorageError")
	}
	a.DisconnectMCP(name)
	if exists {
		_ = a.credentials.Delete(clientCredentialRef(previous))
	}
	return nil
}

func (a *Platform) DisconnectMCP(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if conn := a.clients[name]; conn != nil {
		delete(a.clients, name)
		_ = conn.session.Close()
	}
}

func (a *Platform) ChatTools(id string) []agentrun.ReadTool {
	tools := a.registry.ChatTools(id)
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, conn := range a.clients {
		for _, tool := range conn.chatTools() {
			tools = append(tools, a.registry.chatTool(id, tool.Name, tool))
		}
	}
	return tools
}

func (c *clientConnection) chatTools() []Tool {
	name := c.status.Name
	tools := make([]Tool, 0, len(c.tools)+2)
	for _, external := range c.tools {
		schema, err := json.Marshal(external.InputSchema)
		if err != nil || len(schema) > 8192 {
			continue
		}
		tools = append(tools, Tool{
			Name: clientToolName(name, external.Name, "tool"), Description: external.Description,
			permissionID: externalToolPermissionID(c.status.Config, "tool", external.Name),
			externalName: external.Name, RequiresApproval: true, schema: schema,
			invoke: func(ctx context.Context, args json.RawMessage) (any, error) {
				result, err := c.session.CallTool(ctx, &mcp.CallToolParams{Name: external.Name, Arguments: args})
				if err != nil {
					return nil, errors.New("MCPResultUnknown")
				}
				if result.NeedsInput() {
					return nil, errors.New("MCPAdditionalInputRequired")
				}
				return result, nil
			},
		})
	}
	return append(tools, c.contentTools(name)...)
}

func clientToolName(connection, name, kind string) string {
	wire := sanitizeName("mcp_" + connection + "_" + name)
	identity := sha256.Sum256([]byte(kind + "\x00" + connection + "\x00" + name))
	return wire[:min(len(wire), 50)] + "_" + hex.EncodeToString(identity[:6])
}

func sanitizeName(name string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, name)
}
