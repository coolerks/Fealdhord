package appagent

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/coolerks/Fealdhord/internal/agentrun"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxMCPClientSettings = 512 << 10

type ClientStatus struct {
	ID        string       `json:"id"`
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
	target  ClientConfig
}

func (a *Platform) ConnectMCP(ctx context.Context, config ClientConfig) (ClientStatus, error) {
	a.clientEdit.Lock()
	defer a.clientEdit.Unlock()
	return a.connectMCP(ctx, config)
}

func (a *Platform) connectMCP(ctx context.Context, config ClientConfig) (ClientStatus, error) {
	if err := config.validate(); err != nil {
		return ClientStatus{}, err
	}
	values, err := a.savedClients()
	if err != nil {
		return ClientStatus{}, err
	}
	// Older desktop callers reconnect by name; newer callers send the stable ID.
	requestedID := config.ID
	if config.ID == "" {
		for id, saved := range values {
			if !config.NewConnection && saved.Name == config.Name {
				config.ID = id
				break
			}
		}
		if config.ID == "" {
			if len(values) >= 32 {
				return ClientStatus{}, errors.New("TooManyClients")
			}
			config.ID = "mcp-" + rand.Text()
		}
	}
	previous, exists := values[config.ID]
	if requestedID != "" && !exists {
		return ClientStatus{}, errors.New("MCPClientNotFound")
	}
	if !exists && len(values) >= 32 {
		return ClientStatus{}, errors.New("TooManyClients")
	}
	for id, saved := range values {
		if id != config.ID && strings.EqualFold(saved.Name, config.Name) {
			return ClientStatus{}, errors.New("MCPClientNameExists")
		}
	}
	secrets, err := a.resolveClientSecrets(config, previous)
	if err != nil {
		return ClientStatus{}, err
	}
	effective, expanded, err := expandClientConfig(config, secrets)
	if err != nil {
		return ClientStatus{}, err
	}
	var log *clientLogBuffer
	if config.Command != "" {
		log = newClientLogBuffer(expanded, effective.logSecrets...)
		a.registerClientAttempt(config.AttemptID, log)
	}
	conn, err := openMCPClient(ctx, effective, expanded, log)
	if err != nil {
		if log != nil {
			log.finish()
			log.add("client", "error", err.Error())
		}
		return ClientStatus{}, err
	}
	if err := a.persistClientSecrets(config, values, secrets); err != nil {
		_ = conn.session.Close()
		if log != nil {
			log.finish()
			log.add("client", "error", err.Error())
		}
		return ClientStatus{}, err
	}
	conn.status.Config = values[config.ID].public()
	a.mu.Lock()
	online := a.clients[config.ID]
	a.clients[config.ID] = conn
	if log != nil {
		if a.clientLogs == nil {
			a.clientLogs = make(map[string]*clientLogBuffer)
		}
		a.clientLogs[config.ID] = log
	} else {
		delete(a.clientLogs, config.ID)
	}
	a.mu.Unlock()
	if online != nil {
		_ = online.session.Close()
	}
	go func() {
		_ = conn.session.Wait()
		if log != nil {
			log.finish()
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.clients[config.ID] == conn {
			delete(a.clients, config.ID)
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
	for id, config := range settings {
		if a.clients[id] == nil {
			values = append(values, ClientStatus{ID: id, Name: config.Name, Config: config.public()})
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
	for id, config := range values {
		if config.ID != "" && config.ID != id || !clientIDPattern.MatchString(id) || !validClientName(config.Name) {
			return nil, failure
		}
		config.legacy = config.ID == ""
		config.ID = id
		config.defaults()
		values[id] = config
	}
	return values, nil
}

func (a *Platform) resolveClientSecrets(config, previous ClientConfig) (clientSecrets, error) {
	var secrets clientSecrets
	var saved clientSecrets
	if config.Endpoint != "" && config.Endpoint == previous.Endpoint || config.Command != "" && config.Command == previous.Command {
		var err error
		saved, err = a.loadClientSecrets(previous)
		if err != nil {
			return secrets, err
		}
	}
	var err error
	secrets.Environment, err = clientValues(config.Environment, saved.Environment, false)
	if err != nil {
		return secrets, err
	}
	secrets.Headers, err = clientValues(config.Headers, saved.Headers, true)
	if err != nil {
		return secrets, err
	}
	secrets.Bearer = config.Bearer
	if secrets.Bearer == "" {
		secrets.Bearer = saved.Bearer
	}
	if secrets.Bearer != "" {
		if _, explicit := secrets.Headers["Authorization"]; !explicit {
			secrets.Headers["Authorization"] = "Bearer " + secrets.Bearer
		}
		secrets.Bearer = ""
	}
	secrets.ProxyPassword = config.ProxyPassword
	if secrets.ProxyPassword == "" && config.ProxyPasswordSet {
		if config.ProxyHost != previous.ProxyHost || config.ProxyPort != previous.ProxyPort || saved.ProxyPassword == "" {
			return secrets, errors.New("ClientCredentialUnavailable")
		}
		secrets.ProxyPassword = saved.ProxyPassword
	}
	return secrets, nil
}

func (a *Platform) persistClientSecrets(config ClientConfig, values map[string]ClientConfig, secrets clientSecrets) error {
	id := config.identity()
	previous := values[id]
	config = config.public()
	config.Environment = storedClientValues(secrets.Environment)
	config.Headers = storedClientValues(secrets.Headers)
	config.ProxyPasswordSet = secrets.ProxyPassword != ""
	var parts []string
	if len(secrets.Environment)+len(secrets.Headers) > 0 || secrets.ProxyPassword != "" || secrets.Bearer != "" {
		var err error
		parts, err = encodeClientSecrets(secrets)
		if err != nil {
			return err
		}
		config.SecretRef = "mcp-client-config:" + id + ":" + rand.Text()
		config.SecretParts = len(parts)
	}
	values[id] = config
	data, err := json.Marshal(values)
	if err != nil || len(data) > maxMCPClientSettings {
		return errors.New("ClientSettingsTooLarge")
	}
	for index, part := range parts {
		if a.credentials.Set(clientSecretPart(config.SecretRef, index), part) != nil {
			a.deleteClientSecrets(config)
			return errors.New("CredentialError")
		}
	}
	if err := atomicFile(a.dir, "mcp-clients.json", data); err != nil {
		if config.SecretRef != "" {
			a.deleteClientSecrets(config)
		}
		return errors.New("StorageError")
	}
	a.deleteClientSecrets(previous)
	return nil
}

func storedClientValues(values map[string]string) []ClientValue {
	var fields []ClientValue
	for name, value := range values {
		field := ClientValue{Name: name, Stored: true}
		if value != "" && envReference.FindString(value) == value {
			field.Value = value
		}
		fields = append(fields, field)
	}
	slices.SortFunc(fields, func(a, b ClientValue) int { return strings.Compare(a.Name, b.Name) })
	return fields
}

func (a *Platform) deleteClientSecrets(config ClientConfig) {
	if config.SecretRef != "" {
		for index := range min(config.SecretParts, maxClientSecretParts) {
			_ = a.credentials.Delete(clientSecretPart(config.SecretRef, index))
		}
	} else if config.Endpoint != "" {
		_ = a.credentials.Delete(clientCredentialRef(config))
	}
}

func clientCredentialRef(config ClientConfig) string {
	digest := sha256.Sum256([]byte(config.Endpoint))
	return "mcp-client:" + config.identity() + ":" + hex.EncodeToString(digest[:])
}

func (a *Platform) ForgetMCP(id string) error {
	if !clientIDPattern.MatchString(id) && !validClientName(id) {
		return errClientConfig
	}
	a.clientEdit.Lock()
	defer a.clientEdit.Unlock()
	values, err := a.savedClients()
	if err != nil {
		return err
	}
	if _, ok := values[id]; !ok {
		for key, config := range values {
			if config.Name == id {
				id = key
				break
			}
		}
	}
	previous := values[id]
	delete(values, id)
	if err := atomicJSON(a.dir, "mcp-clients.json", values); err != nil {
		return errors.New("StorageError")
	}
	a.disconnectMCP(id)
	a.mu.Lock()
	delete(a.clientLogs, id)
	a.mu.Unlock()
	a.deleteClientSecrets(previous)
	return nil
}

func (a *Platform) DisconnectMCP(id string) {
	a.clientEdit.Lock()
	defer a.clientEdit.Unlock()
	a.disconnectMCP(id)
}

func (a *Platform) disconnectMCP(id string) {
	a.mu.Lock()
	conn := a.clients[id]
	if conn == nil {
		for key, candidate := range a.clients {
			if candidate.status.Name == id {
				id, conn = key, candidate
				break
			}
		}
	}
	delete(a.clients, id)
	a.mu.Unlock()
	if conn != nil {
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
	id := c.status.ID
	tools := make([]Tool, 0, len(c.tools)+2)
	for _, external := range c.tools {
		schema, err := json.Marshal(external.InputSchema)
		if err != nil || len(schema) > 8192 {
			continue
		}
		tools = append(tools, Tool{
			Name: clientToolName(id, external.Name, "tool"), Description: external.Description,
			permissionID: externalToolPermissionID(c.target, "tool", external.Name),
			externalName: external.Name, RequiresApproval: true, schema: schema,
			invoke: func(ctx context.Context, args json.RawMessage) (any, error) {
				ctx, cancel := toolDeadline(ctx, c.target.ToolTimeout)
				defer cancel()
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
	return append(tools, c.contentTools(c.status.Name)...)
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
