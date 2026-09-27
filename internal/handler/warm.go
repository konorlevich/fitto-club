package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
)

// Warm renders every indexable URL once through the real handler, filling
// the page cache before the first visitor arrives. A page that fails to
// render fails the boot.
func (s *Server) Warm(h http.Handler) (int, error) {
	n := 0
	for _, lang := range s.Cfg.Locales {
		for _, e := range s.entries(s.Store.Current()) {
			req := httptest.NewRequest(http.MethodGet, "/"+lang+e.Path, nil)
			req.Header.Set("Accept-Encoding", "br")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				return n, fmt.Errorf("pre-rendering /%s%s: status %d", lang, e.Path, rec.Code)
			}
			n++
		}
	}
	return n, nil
}
