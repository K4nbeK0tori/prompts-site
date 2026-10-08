package server

import (
	"net/http"
	"path"
	"strings"
)

// handleStatic 提供内嵌前端资源，并为 SPA 做 index.html 兜底。
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeErr(w, http.StatusNotFound, "接口不存在")
		return
	}
	if s.static == nil {
		writeErr(w, http.StatusServiceUnavailable, "前端资源未打包进二进制")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeErr(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	clean := path.Clean("/" + r.URL.Path)
	name := strings.TrimPrefix(clean, "/")
	if name == "" {
		name = "index.html"
	}

	if f, err := s.static.Open(name); err == nil {
		f.Close()
		if strings.HasPrefix(clean, "/assets/") {
			// Vite 产物带内容哈希，可长期缓存。
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		http.FileServerFS(s.static).ServeHTTP(w, r)
		return
	}

	// 前端路由兜底：任何未知路径都返回 index.html。
	clone := r.Clone(r.Context())
	clone.URL.Path = "/"
	clone.URL.RawPath = ""
	w.Header().Set("Cache-Control", "no-cache")
	http.FileServerFS(s.static).ServeHTTP(w, clone)
}
