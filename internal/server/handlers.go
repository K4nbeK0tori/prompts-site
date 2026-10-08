package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"prompts-site/internal/store"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.st.Ping(); err != nil {
		writeErr(w, http.StatusServiceUnavailable, "数据库不可用")
		return
	}
	writeOK(w, map[string]any{"status": "ok", "version": s.version})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]any{
		"authenticated": s.authenticated(r),
		"version":       s.version,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := ClientIP(r)
	if ok, wait := s.limiter.allow(ip); !ok {
		writeErr(w, http.StatusTooManyRequests,
			fmt.Sprintf("尝试过于频繁，请 %.0f 分钟后再试", wait.Minutes()+0.5))
		return
	}

	var body struct {
		Password string `json:"password"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	body.Password = strings.TrimSpace(body.Password)
	if body.Password == "" {
		writeErr(w, http.StatusBadRequest, "请输入密码")
		return
	}

	encoded, ok, err := s.st.Setting("admin_password_hash")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "读取凭据失败")
		return
	}
	if !ok || encoded == "" {
		writeErr(w, http.StatusInternalServerError, "管理员密码尚未初始化")
		return
	}
	if !verifyPassword(encoded, body.Password) {
		s.limiter.fail(ip)
		log.Printf("登录失败 ip=%s", ip)
		writeErr(w, http.StatusUnauthorized, "密码不正确")
		return
	}

	s.limiter.reset(ip)
	s.setSessionCookie(w, r)
	log.Printf("登录成功 ip=%s", ip)
	writeOK(w, map[string]any{"authenticated": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.clearSessionCookie(w, r)
	writeOK(w, map[string]any{"authenticated": false})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	st, err := s.st.Stats()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, st)
}

func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
	tags, err := s.st.Tags()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]any{"tags": tags})
}

func parseID(r *http.Request) (int64, bool) {
	raw := r.PathValue("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func splitQueryList(values []string) []string {
	var out []string
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func (s *Server) handleListPrompts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	size, _ := strconv.Atoi(q.Get("size"))

	f := store.Filter{
		Query:    q.Get("q"),
		Tags:     splitQueryList(q["tag"]),
		Source:   q.Get("source"),
		Status:   q.Get("status"),
		Favorite: q.Get("favorite") == "1" || q.Get("favorite") == "true",
		Sort:     q.Get("sort"),
		Page:     page,
		Size:     size,
	}
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.Size <= 0 {
		f.Size = 24
	}

	items, total, err := s.st.List(f)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	pages := (total + f.Size - 1) / f.Size
	writeOK(w, map[string]any{
		"items": items,
		"total": total,
		"page":  f.Page,
		"size":  f.Size,
		"pages": pages,
	})
}

func (s *Server) handleGetPrompt(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	p, err := s.st.Get(id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "记录不存在")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, p)
}

func (s *Server) handleCreatePrompt(w http.ResponseWriter, r *http.Request) {
	var in store.PromptInput
	if !decodeBody(w, r, &in) {
		return
	}
	p, err := s.st.Create(in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, apiEnvelope{OK: true, Data: p})
}

func (s *Server) handleUpdatePrompt(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	var in store.PromptInput
	if !decodeBody(w, r, &in) {
		return
	}
	p, err := s.st.Update(id, in)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "记录不存在")
		return
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeOK(w, p)
}

func (s *Server) handleDeletePrompt(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	if err := s.st.Delete(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "记录不存在")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]any{"deleted": id})
}

func (s *Server) handleFavoritePrompt(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	var body struct {
		Favorite *bool `json:"favorite"`
	}
	if r.ContentLength > 0 {
		if !decodeBody(w, r, &body) {
			return
		}
	}
	cur, err := s.st.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "记录不存在")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	target := !cur.Favorite
	if body.Favorite != nil {
		target = *body.Favorite
	}
	p, err := s.st.SetFavorite(id, target)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, p)
}

func (s *Server) handleEnabledPrompt(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if r.ContentLength > 0 {
		if !decodeBody(w, r, &body) {
			return
		}
	}
	cur, err := s.st.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "记录不存在")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	target := !cur.Enabled
	if body.Enabled != nil {
		target = *body.Enabled
	}
	p, err := s.st.SetEnabled(id, target)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, p)
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	items, err := s.st.All()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	filename := "prompts-export-" + time.Now().Format("20060102-150405") + ".json"
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	writeOK(w, map[string]any{
		"exported_at": time.Now().Format(time.RFC3339),
		"count":       len(items),
		"items":       items,
	})
}
