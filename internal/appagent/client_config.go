package appagent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/zalando/go-keyring"
)

type ClientValue struct {
	Name   string `json:"name"`
	Value  string `json:"value,omitempty"`
	Stored bool   `json:"stored,omitempty"`
}

type ClientConfig struct {
	Paused            bool          `json:"paused,omitempty"`
	ToolHints         []string      `json:"toolHints,omitempty"`
	ID                string        `json:"id,omitempty"`
	Name              string        `json:"name"`
	Endpoint          string        `json:"endpoint"`
	Command           string        `json:"command"`
	Args              []string      `json:"args"`
	Environment       []ClientValue `json:"environment,omitempty"`
	WorkingDirectory  string        `json:"workingDirectory,omitempty"`
	Headers           []ClientValue `json:"headers,omitempty"`
	StartupTimeout    int           `json:"startupTimeout,omitempty"`
	ConnectionTimeout int           `json:"connectionTimeout,omitempty"`
	ToolTimeout       int           `json:"toolTimeout,omitempty"`
	LogLevel          string        `json:"logLevel,omitempty"`
	ProxyHost         string        `json:"proxyHost,omitempty"`
	ProxyPort         string        `json:"proxyPort,omitempty"`
	ProxyUsername     string        `json:"proxyUsername,omitempty"`
	ProxyPassword     string        `json:"proxyPassword,omitempty"`
	ProxyPasswordSet  bool          `json:"proxyPasswordSet,omitempty"`
	SkipTLSVerify     bool          `json:"skipTLSVerify,omitempty"`
	ConnectionRetries *int          `json:"connectionRetries,omitempty"`
	Bearer            string        `json:"bearer,omitempty"`
	AttemptID         string        `json:"attemptId,omitempty"`
	NewConnection     bool          `json:"newConnection,omitempty"`
	legacy            bool
	logSecrets        []string
	SecretRef         string `json:"secretRef,omitempty"`
	SecretParts       int    `json:"secretParts,omitempty"`
}

type clientSecrets struct {
	Environment   map[string]string
	Headers       map[string]string
	ProxyPassword string
	Bearer        string
}

var (
	clientName      = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`) // Existing MCP server grant names.
	clientIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
	envNamePattern  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	envReference    = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
	errClientConfig = errors.New("InvalidClientSettings")
)

func (c ClientConfig) identity() string {
	if c.ID != "" {
		return c.ID
	}
	return c.Name
}

func (c ClientConfig) public() ClientConfig {
	c.SecretRef, c.AttemptID, c.Bearer, c.ProxyPassword = "", "", "", ""
	c.NewConnection = false
	c.SecretParts, c.logSecrets = 0, nil
	return c
}

func validClientName(name string) bool {
	return utf8.ValidString(name) && strings.TrimSpace(name) == name && name != "" &&
		utf8.RuneCountInString(name) <= 64 && !strings.ContainsFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsMark(r) && !unicode.IsNumber(r) && !strings.ContainsRune(" _-.·", r)
	})
}

func (c *ClientConfig) defaults() {
	if c.ToolTimeout == 0 {
		c.ToolTimeout = 120
	}
	if c.Command != "" {
		if c.StartupTimeout == 0 {
			c.StartupTimeout = 15
		}
		if c.LogLevel == "" {
			c.LogLevel = "warning"
		}
	} else {
		if c.ConnectionTimeout == 0 {
			c.ConnectionTimeout = 120
		}
		if c.ConnectionRetries == nil {
			retries := 3
			c.ConnectionRetries = &retries
		}
	}
}

func (c *ClientConfig) validate() error {
	c.Name = strings.TrimSpace(c.Name)
	if !validClientName(c.Name) || (c.Command == "") == (c.Endpoint == "") ||
		(c.ID != "" && (!clientIDPattern.MatchString(c.ID) || c.NewConnection)) ||
		(c.AttemptID != "" && !clientIDPattern.MatchString(c.AttemptID)) ||
		len(c.Bearer) > 4096 || !validHeaderValue(c.Bearer) ||
		len(c.Args) > 128 || len(c.Environment) > 64 || len(c.Headers) > 64 ||
		len(c.ProxyPassword) > 4096 {
		return errClientConfig
	}
	for _, timeout := range []int{c.StartupTimeout, c.ConnectionTimeout, c.ToolTimeout} {
		if timeout < 0 || timeout > 3600 {
			return errClientConfig
		}
	}
	if c.ConnectionRetries != nil && (*c.ConnectionRetries < 0 || *c.ConnectionRetries > 10) {
		return errClientConfig
	}
	if c.LogLevel != "" && !slices.Contains([]string{"debug", "info", "notice", "warning", "error", "critical", "alert", "emergency"}, c.LogLevel) {
		return errClientConfig
	}
	if c.Command != "" {
		c.Headers, c.ConnectionRetries = nil, nil
		c.ConnectionTimeout, c.SkipTLSVerify = 0, false
		c.Bearer, c.ProxyHost, c.ProxyPort, c.ProxyUsername, c.ProxyPassword, c.ProxyPasswordSet = "", "", "", "", "", false
	} else {
		c.Args, c.Environment = nil, nil
		c.WorkingDirectory, c.StartupTimeout, c.LogLevel = "", 0, ""
	}
	c.defaults()
	return nil
}

func expandClientConfig(c ClientConfig, secrets clientSecrets) (ClientConfig, clientSecrets, error) {
	var missing string
	expand := func(value string) string {
		return envReference.ReplaceAllStringFunc(value, func(ref string) string {
			name := ref[2 : len(ref)-1]
			result, ok := os.LookupEnv(name)
			if !ok && missing == "" {
				missing = name
			}
			if ok && len(c.logSecrets) < 256 && !slices.Contains(c.logSecrets, result) {
				c.logSecrets = append(c.logSecrets, result)
			}
			return result
		})
	}
	for _, field := range []*string{&c.Endpoint, &c.Command, &c.WorkingDirectory, &c.ProxyHost, &c.ProxyPort, &c.ProxyUsername, &secrets.ProxyPassword, &secrets.Bearer} {
		*field = expand(*field)
	}
	c.Args = slices.Clone(c.Args)
	for index, arg := range c.Args {
		c.Args[index] = expand(arg)
	}
	expandValues := func(values map[string]string) map[string]string {
		result := make(map[string]string, len(values))
		for key, value := range values {
			result[key] = expand(value)
		}
		return result
	}
	secrets.Environment, secrets.Headers = expandValues(secrets.Environment), expandValues(secrets.Headers)
	if missing != "" {
		return c, secrets, fmt.Errorf("MissingEnvironmentVariable: %s", missing)
	}
	for _, value := range append(slices.Clone(c.Args), c.Command, c.WorkingDirectory) {
		if strings.ContainsRune(value, 0) {
			return c, secrets, errClientConfig
		}
	}
	for _, value := range secrets.Environment {
		if strings.ContainsRune(value, 0) {
			return c, secrets, errClientConfig
		}
	}
	if !validHeaderValue(secrets.Bearer) {
		return c, secrets, errClientConfig
	}
	for _, value := range secrets.Headers {
		if !validHeaderValue(value) {
			return c, secrets, errClientConfig
		}
	}
	return c, secrets, nil
}

func validHeaderValue(value string) bool {
	return !strings.ContainsFunc(value, func(r rune) bool { return (r < 32 && r != '\t') || r == 127 })
}

func clientValues(entries []ClientValue, previous map[string]string, headers bool) (map[string]string, error) {
	result := make(map[string]string, len(entries))
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name)
		valid := envNamePattern.MatchString(name)
		if headers {
			valid = name != "" && !strings.ContainsFunc(name, func(r rune) bool {
				return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("!#$%&'*+-.^`_|~", r))
			})
			name = http.CanonicalHeaderKey(name)
			switch name {
			case "Host", "Connection", "Content-Length", "Content-Type", "Accept", "Mcp-Session-Id", "Mcp-Protocol-Version", "Last-Event-Id", "Proxy-Authorization", "Transfer-Encoding", "Upgrade":
				valid = false
			}
		}
		if !valid || len(name) > 128 || len(entry.Value) > 8192 {
			return nil, errClientConfig
		}
		if _, duplicate := result[name]; duplicate {
			return nil, errClientConfig
		}
		value := entry.Value
		if entry.Stored && value == "" {
			var ok bool
			value, ok = previous[name]
			if !ok {
				return nil, errors.New("ClientCredentialUnavailable")
			}
		}
		result[name] = value
	}
	return result, nil
}

func (a *Platform) loadClientSecrets(c ClientConfig) (clientSecrets, error) {
	var secrets clientSecrets
	if c.SecretRef != "" {
		prefix := "mcp-client-config:" + c.identity() + ":"
		if !strings.HasPrefix(c.SecretRef, prefix) || !clientIDPattern.MatchString(strings.TrimPrefix(c.SecretRef, prefix)) {
			return secrets, errors.New("CredentialError")
		}
		if c.SecretParts < 1 || c.SecretParts > maxClientSecretParts {
			return secrets, errors.New("CredentialError")
		}
		var encoded strings.Builder
		for index := range c.SecretParts {
			part, err := a.credentials.Get(clientSecretPart(c.SecretRef, index))
			if err != nil || len(part) > clientSecretPartSize {
				return secrets, errors.New("ClientCredentialUnavailable")
			}
			encoded.WriteString(part)
		}
		raw, err := base64.StdEncoding.DecodeString(encoded.String())
		if err != nil || json.Unmarshal(raw, &secrets) != nil {
			return secrets, errors.New("ClientCredentialUnavailable")
		}
	} else if c.legacy && c.Endpoint != "" {
		var err error
		secrets.Bearer, err = a.credentials.Get(clientCredentialRef(c))
		if err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return secrets, errors.New("CredentialError")
		}
	}
	return secrets, nil
}

func clientProxy(c ClientConfig, password string) (*url.URL, error) {
	host, port := strings.TrimSpace(c.ProxyHost), strings.TrimSpace(c.ProxyPort)
	if host == "" && port == "" && c.ProxyUsername == "" && password == "" {
		return nil, nil
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 || host == "" || strings.ContainsAny(host, "/?#@%[]") ||
		strings.ContainsFunc(host, unicode.IsSpace) || (strings.Contains(host, ":") && net.ParseIP(host) == nil) ||
		(password != "" && c.ProxyUsername == "") {
		return nil, errors.New("InvalidMCPProxy")
	}
	proxy := &url.URL{Scheme: "http", Host: net.JoinHostPort(host, port)}
	if c.ProxyUsername != "" {
		proxy.User = url.UserPassword(c.ProxyUsername, password)
	}
	return proxy, nil
}

func toolDeadline(ctx context.Context, seconds int) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
}

// Windows Credential Manager limits the size of each generic credential.
const clientSecretPartSize = 1800
const maxClientSecretParts = 768

func clientSecretPart(ref string, index int) string {
	return ref + ":" + strconv.Itoa(index)
}

func encodeClientSecrets(secrets clientSecrets) ([]string, error) {
	raw, err := json.Marshal(secrets)
	if err != nil {
		return nil, errors.New("CredentialError")
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	if len(encoded) > maxClientSecretParts*clientSecretPartSize {
		return nil, errors.New("ClientSettingsTooLarge")
	}
	parts := make([]string, 0, (len(encoded)+clientSecretPartSize-1)/clientSecretPartSize)
	for len(encoded) > 0 {
		end := min(len(encoded), clientSecretPartSize)
		parts = append(parts, encoded[:end])
		encoded = encoded[end:]
	}
	return parts, nil
}
