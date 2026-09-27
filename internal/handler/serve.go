package handler

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/konorlevich/fitto-club/internal/render"
	"github.com/konorlevich/fitto-club/internal/store"
)

// A rendered page is cached as bytes, precompressed once, and served from
// memory with ETag/304 (checklist §2). The key carries everything that can
// change the bytes: the URL, the content generation, and the clock bucket -
// "open until 23:00" and "next class" depend on the time, so the bucket is
// the hour plus the open/closed state.
type cached struct {
	status int
	body   []byte
	gz, br []byte
	etag   string
	mod    time.Time
	bucket string
}

type pageCache struct {
	mu sync.RWMutex
	m  map[string]*cached
}

func newPageCache() *pageCache { return &pageCache{m: map[string]*cached{}} }

func bucketOf(snap *store.Snapshot, now time.Time) string {
	return fmt.Sprintf("%d|%s|%s", snap.Gen, now.Format("2006-01-02T15"), snap.Hours.StatusAt(now).Key())
}

func cacheKey(path, speaks, tag string, snap *store.Snapshot, now time.Time) string {
	return path + "?" + speaks + "&" + tag + "|" + bucketOf(snap, now)
}

func (c *pageCache) get(k string) (*cached, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.m[k]
	return e, ok
}

func (c *pageCache) put(k string, e *cached) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Entries from an older bucket can never be hit again; drop them.
	for key, old := range c.m {
		if old.bucket != e.bucket {
			delete(c.m, key)
		}
	}
	c.m[k] = e
}

// serveCached answers from the cache; false means render and serve.
func (s *Server) serveCached(w http.ResponseWriter, r *http.Request, key, lang string) bool {
	if key == "" {
		return false
	}
	e, ok := s.cache.get(key)
	if !ok {
		return false
	}
	s.setLocaleCookie(w, lang)
	s.write(w, r, e)
	return true
}

func (s *Server) renderAndServe(w http.ResponseWriter, r *http.Request, key, name string, p *render.Page, status int) {
	t, ok := s.Tmpl[name]
	if !ok {
		s.Log.WithField("template", name).Error("unknown template")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", p); err != nil {
		s.Log.WithError(err).WithField("template", name).Error("template execution failed")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	body := buf.Bytes()
	sum := sha256.Sum256(body)
	mod, _ := time.Parse("2006-01-02", p.Snap.Date(pathKey(p.Path)))
	e := &cached{
		status: status, body: body, etag: `"` + hex.EncodeToString(sum[:10]) + `"`,
		mod: mod, bucket: bucketOf(p.Snap, p.Now),
		gz: gzipBytes(body), br: brotliBytes(body),
	}
	if key != "" && status == http.StatusOK {
		s.cache.put(key, e)
	}
	s.write(w, r, e)
}

// pathKey maps a page path to its lastmod key: a coach page is keyed by its
// own path, everything else by the section.
func pathKey(p string) string {
	if p == "" {
		return "/"
	}
	return p
}

func (s *Server) write(w http.ResponseWriter, r *http.Request, e *cached) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-cache")
	h.Add("Vary", "Accept-Encoding")
	h.Set("ETag", e.etag)
	if !e.mod.IsZero() {
		h.Set("Last-Modified", e.mod.UTC().Format(http.TimeFormat))
	}
	if e.status == http.StatusOK && matchETag(r.Header.Get("If-None-Match"), e.etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	body := e.body
	switch enc := r.Header.Get("Accept-Encoding"); {
	case strings.Contains(enc, "br") && e.br != nil:
		h.Set("Content-Encoding", "br")
		body = e.br
	case strings.Contains(enc, "gzip") && e.gz != nil:
		h.Set("Content-Encoding", "gzip")
		body = e.gz
	}
	h.Set("Content-Length", fmt.Sprint(len(body)))
	w.WriteHeader(e.status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

func matchETag(header, etag string) bool {
	for part := range strings.SplitSeq(header, ",") {
		p := strings.TrimPrefix(strings.TrimSpace(part), "W/")
		if p == etag || p == "*" {
			return true
		}
	}
	return false
}

func gzipBytes(b []byte) []byte {
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	_, _ = zw.Write(b)
	_ = zw.Close()
	return buf.Bytes()
}

func brotliBytes(b []byte) []byte {
	var buf bytes.Buffer
	bw := brotli.NewWriterLevel(&buf, 9)
	_, _ = bw.Write(b)
	_ = bw.Close()
	return buf.Bytes()
}
