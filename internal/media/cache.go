package media

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const maxDiskBytes int64 = 192 << 20
const maxMemoryBytes int64 = 64 << 20
const ttl = 7 * 24 * time.Hour

type memoryEntry struct {
	data            []byte
	access, created time.Time
}
type cache struct {
	mu          sync.Mutex
	directory   string
	memory      map[string]memoryEntry
	memoryBytes int64
	now         func() time.Time
}

func newCache(directory string) (*cache, error) {
	if directory != "" {
		if err := os.MkdirAll(directory, 0700); err != nil {
			return nil, err
		}
	}
	return &cache{directory: directory, memory: map[string]memoryEntry{}, now: time.Now}, nil
}
func cacheKey(raw string) string {
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}
func validKey(key string) bool {
	if len(key) != 64 {
		return false
	}
	_, err := hex.DecodeString(key)
	return err == nil
}
func (c *cache) get(key string, disk bool) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if entry, ok := c.memory[key]; ok {
		if now.Sub(entry.created) < time.Minute {
			entry.access = now
			c.memory[key] = entry
			return entry.data, true
		}
		delete(c.memory, key)
		c.memoryBytes -= int64(len(entry.data))
	}
	if !disk || c.directory == "" || !validKey(key) {
		return nil, false
	}
	name := filepath.Join(c.directory, key)
	info, err := os.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() > (8<<20)+8 {
		return nil, false
	}
	body, err := os.ReadFile(name)
	if err != nil || len(body) < 8 {
		return nil, false
	}
	created := int64(binary.BigEndian.Uint64(body[:8]))
	if now.Sub(time.UnixMilli(created)) >= ttl {
		os.Remove(name)
		return nil, false
	}
	os.Chtimes(name, now, now)
	return body[8:], true
}
func (c *cache) put(key string, body []byte, disk bool) error {
	if !validKey(key) || len(body) > 8<<20 {
		return errors.New("媒体缓存参数无效")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if disk && c.directory != "" {
		file, err := os.CreateTemp(c.directory, "pending-")
		if err != nil {
			return err
		}
		name := file.Name()
		defer os.Remove(name)
		stamp := make([]byte, 8)
		binary.BigEndian.PutUint64(stamp, uint64(now.UnixMilli()))
		if err = file.Chmod(0600); err == nil {
			_, err = file.Write(stamp)
		}
		if err == nil {
			_, err = file.Write(body)
		}
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
		if err = os.Rename(name, filepath.Join(c.directory, key)); err != nil {
			return err
		}
		c.pruneDisk()
		return nil
	}
	if old, ok := c.memory[key]; ok {
		c.memoryBytes -= int64(len(old.data))
	}
	c.memory[key] = memoryEntry{data: append([]byte(nil), body...), access: now, created: now}
	c.memoryBytes += int64(len(body))
	for c.memoryBytes > maxMemoryBytes {
		oldest := ""
		var at time.Time
		for name, entry := range c.memory {
			if oldest == "" || entry.access.Before(at) {
				oldest = name
				at = entry.access
			}
		}
		c.memoryBytes -= int64(len(c.memory[oldest].data))
		delete(c.memory, oldest)
	}
	return nil
}
func (c *cache) pruneDisk() {
	entries, _ := os.ReadDir(c.directory)
	type diskEntry struct {
		name string
		size int64
		at   time.Time
	}
	files := []diskEntry{}
	var total int64
	for _, entry := range entries {
		if !validKey(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		total += info.Size()
		files = append(files, diskEntry{entry.Name(), info.Size(), info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].at.Before(files[j].at) })
	for _, file := range files {
		if total <= maxDiskBytes {
			break
		}
		if os.Remove(filepath.Join(c.directory, file.name)) == nil {
			total -= file.size
		}
	}
}
func (c *cache) clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.memory = map[string]memoryEntry{}
	c.memoryBytes = 0
	if c.directory == "" {
		return nil
	}
	entries, err := os.ReadDir(c.directory)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if validKey(entry.Name()) || strings.HasPrefix(entry.Name(), "pending-") {
			if err := os.Remove(filepath.Join(c.directory, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
