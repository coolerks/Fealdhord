package media

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/netpolicy"
)

type inlineRepository interface {
	ReaderResource(context.Context, string) (database.ReaderResource, error)
}

func (s *Service) inlineAllowed(ctx context.Context, record database.ReaderResource) bool {
	repo, ok := s.repository.(inlineRepository)
	if !ok || s.policy == nil {
		return false
	}
	policy := s.policy(ctx)
	if policy.RemoteImages == "never" {
		return false
	}
	current, err := repo.ReaderResource(ctx, record.ID)
	return err == nil && current.Kind == "image" && current.ArticleHash == record.ArticleHash && (policy.RemoteImages == "always" || (policy.RemoteImages == "subscriptions" && !current.Archived))
}
func (s *Service) loadInline(ctx context.Context, id string) ([]byte, string, error) {
	repo, ok := s.repository.(inlineRepository)
	if !ok || s.fetcher == nil {
		return nil, "", errUnavailable
	}
	record, err := repo.ReaderResource(ctx, id)
	if err != nil || !s.inlineAllowed(ctx, record) {
		return nil, "", errUnavailable
	}
	key := cacheKey(record.URL)
	if body, ok := s.cache.get(key, s.policy(ctx).CacheEnabled); ok {
		_, kind, err := imageHeader(body)
		return body, "image/" + kind, err
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, "", context.Canceled
	}
	flightID := "inline:" + id
	if old := s.active[flightID]; old != nil {
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-old.done:
			return old.body, old.mime, old.err
		}
	}
	requestCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	call := &flight{done: make(chan struct{}), cancel: cancel}
	s.active[flightID] = call
	s.wg.Add(1)
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		delete(s.active, flightID)
		close(call.done)
		s.mu.Unlock()
		s.wg.Done()
	}()
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-requestCtx.Done():
		call.err = requestCtx.Err()
		return nil, "", call.err
	}
	response, err := s.fetcher.Fetch(requestCtx, record.URL, netpolicy.Request{MaxBytes: netpolicy.ImageLimit, Timeout: 8 * time.Second, Accept: "image/png,image/jpeg,image/webp"})
	if err == nil && response.Status != http.StatusOK {
		err = errUnavailable
	}
	if err == nil && len(response.Body) > int(netpolicy.ImageLimit) {
		err = errUnavailable
	}
	if err == nil {
		_, call.mime, err = imageInfo(response.Body)
		if err == nil && (requestCtx.Err() != nil || !s.inlineAllowed(ctx, record)) {
			err = errUnavailable
		}
		if err == nil {
			if diskAllowed(Policy{CacheEnabled: true}, record.URL, response.Header) && diskAllowed(Policy{CacheEnabled: true}, response.URL, response.Header) {
				err = s.cache.put(key, response.Body, diskAllowed(s.policy(ctx), record.URL, response.Header))
			}
			if err == nil {
				call.body = response.Body
			}
		}
	}
	call.err = err
	return call.body, call.mime, call.err
}
func (s *Service) serveInline(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/media/inline/")
	if len(id) != 64 {
		http.NotFound(w, r)
		return
	}
	for _, char := range id {
		if !(char >= 'a' && char <= 'f' || char >= '0' && char <= '9') {
			http.NotFound(w, r)
			return
		}
	}
	body, mime, err := s.loadInline(r.Context(), id)
	repo, ok := s.repository.(inlineRepository)
	if err != nil || !ok {
		http.NotFound(w, r)
		return
	}
	record, err := repo.ReaderResource(r.Context(), id)
	if err != nil || !s.inlineAllowed(r.Context(), record) {
		http.NotFound(w, r)
		return
	}
	if r.Context().Err() != nil {
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method == "GET" {
		w.Write(body)
	}
}
