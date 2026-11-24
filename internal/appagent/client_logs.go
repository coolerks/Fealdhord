package appagent

import (
	"bytes"
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	mcpLogSize  = 4 << 10
	mcpLogCount = 100
)

type MCPClientLog struct {
	Time    time.Time `json:"time"`
	Source  string    `json:"source"`
	Level   string    `json:"level,omitempty"`
	Message string    `json:"message"`
}

type clientLogBuffer struct {
	mu        sync.Mutex
	entries   []MCPClientLog
	line      []byte
	truncated bool
	secrets   []string
}

var logCredential = regexp.MustCompile(`(?i)(["']?(?:authorization|proxy-authorization|cookie|password|passwd|api[-_]?key|access[-_]?token|secret|token)["']?\s*[:=]\s*).*`)

func newClientLogBuffer(secrets clientSecrets, references ...string) *clientLogBuffer {
	log := &clientLogBuffer{}
	add := func(value string) {
		for _, line := range strings.Split(value, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !slices.Contains(log.secrets, line) {
				log.secrets = append(log.secrets, line)
				if encoded, err := json.Marshal(line); err == nil && len(encoded) > 2 {
					escaped := string(encoded[1 : len(encoded)-1])
					if escaped != line {
						log.secrets = append(log.secrets, escaped)
					}
				}
			}
		}
	}
	for _, value := range secrets.Environment {
		add(value)
	}
	for _, value := range secrets.Headers {
		add(value)
	}
	for _, value := range references {
		add(value)
	}
	add(secrets.ProxyPassword)
	add(secrets.Bearer)
	slices.SortFunc(log.secrets, func(a, b string) int { return len(b) - len(a) })
	return log
}

func (l *clientLogBuffer) add(source, level, value string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.append(source, level, value)
}

func (l *clientLogBuffer) append(source, level, value string) {
	if len(value) > mcpLogSize {
		value = "日志行过长，内容已省略。"
	} else {
		value = logCredential.ReplaceAllString(value, "${1}[已遮盖]")
		for _, secret := range l.secrets {
			value = strings.ReplaceAll(value, secret, "[已遮盖]")
		}
	}
	if len(value) > mcpLogSize {
		value = "日志行过长，内容已省略。"
	}
	value = strings.TrimSpace(strings.ToValidUTF8(value, "�"))
	if value == "" {
		return
	}
	if len(l.entries) == mcpLogCount {
		copy(l.entries, l.entries[1:])
		l.entries = l.entries[:mcpLogCount-1]
	}
	l.entries = append(l.entries, MCPClientLog{Time: time.Now(), Source: source, Level: level, Message: value})
}

func (l *clientLogBuffer) Write(data []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for rest := data; len(rest) > 0; {
		before, after, newline := bytes.Cut(rest, []byte("\n"))
		if len(l.line)+len(before) > mcpLogSize {
			l.truncated = true
		}
		if !l.truncated {
			l.line = append(l.line, before...)
		}
		if newline {
			l.flushLine()
		}
		rest = after
	}
	return len(data), nil
}

func (l *clientLogBuffer) flushLine() {
	if l.truncated {
		l.append("stderr", "", "日志行过长，内容已省略。")
	} else if len(l.line) > 0 {
		l.append("stderr", "", string(l.line))
	}
	l.line, l.truncated = l.line[:0], false
}

func (l *clientLogBuffer) finish() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.flushLine()
}

func (l *clientLogBuffer) snapshot() []MCPClientLog {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]MCPClientLog{}, l.entries...)
}

func (a *Platform) registerClientAttempt(id string, log *clientLogBuffer) {
	if id == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.clientAttempts == nil {
		a.clientAttempts = make(map[string]*clientLogBuffer)
	}
	a.attemptOrder = slices.DeleteFunc(a.attemptOrder, func(entry string) bool { return entry == id })
	if len(a.attemptOrder) >= 16 {
		delete(a.clientAttempts, a.attemptOrder[0])
		a.attemptOrder = a.attemptOrder[1:]
	}
	a.attemptOrder = append(a.attemptOrder, id)
	a.clientAttempts[id] = log
}

func (a *Platform) MCPClientLogs(id string, attempt bool) []MCPClientLog {
	a.mu.Lock()
	log := a.clientLogs[id]
	if attempt {
		log = a.clientAttempts[id]
	}
	a.mu.Unlock()
	if log == nil {
		return []MCPClientLog{}
	}
	return log.snapshot()
}
