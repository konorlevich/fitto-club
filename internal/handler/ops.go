package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type statusWriter struct {
	http.ResponseWriter
	status int
	n      int
}

func (w *statusWriter) WriteHeader(c int) {
	if w.status == 0 {
		w.status = c
	}
	w.ResponseWriter.WriteHeader(c)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.n += n
	return n, err
}

// accessLog emits exactly one JSON object per request to stdout. It never
// logs cookies, bodies or query strings (a query could carry anything).
func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rid := r.Header.Get("X-Request-Id")
		if rid == "" {
			var b [6]byte
			_, _ = rand.Read(b[:])
			rid = hex.EncodeToString(b[:])
		}
		w.Header().Set("X-Request-Id", rid)
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		if sw.status == 0 {
			sw.status = http.StatusOK
		}
		ip := r.Header.Get("X-Forwarded-For")
		if i := strings.IndexByte(ip, ','); i > 0 {
			ip = ip[:i]
		}
		if ip == "" {
			ip = r.RemoteAddr
		}
		s.Log.WithFields(logrus.Fields{
			"method": r.Method, "path": r.URL.Path, "status": sw.status, "bytes": sw.n,
			"duration_ms": time.Since(start).Milliseconds(), "ip": ip,
			"referer": r.Referer(), "ua": r.UserAgent(), "request_id": rid,
		}).Info("request")
	})
}
