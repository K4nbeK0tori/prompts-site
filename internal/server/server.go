package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"prompts-site/internal/store"
)

const (
	sessionCookieName = "psess"
	sessionTTL        = 30 * 24 * time.Hour
	pbkdf2Iterations  = 120000
	pbkdf2KeyLen      = 32
)

// Options 是构造 Server 所需的依赖。
type Options struct {
	Store   *store.Store
	Static  fs.FS
	Version string
}

// Server 承载全部 HTTP 路由。
type Server struct {
	st      *store.Store
	static  fs.FS
	version string
	secret  []byte
	epoch   int64
	limiter *rateLimiter
	mux     *http.ServeMux
}

// New 构造 Server，并准备会话密钥。
func New(o Options) (*Server, error) {
	if o.Store == nil {
		return nil, errors.New("store 不能为空")
	}
	s := &Server{
		st:      o.Store,
		static:  o.Static,
		version: o.Version,
		limiter: newRateLimiter(),
		mux:     http.NewServeMux(),
	}
	if err := s.loadSecret(); err != nil {
		return nil, err
	}
	epoch, err := s.st.AuthEpoch()
	if err != nil {
		return nil, err
	}
	s.epoch = epoch
	s.routes()
	return s, nil
}

func (s *Server) loadSecret() error {
	v, ok, err := s.st.Setting("session_secret")
	if err != nil {
		return err
	}
	if ok && v != "" {
		raw, err := base64.StdEncoding.DecodeString(v)
		if err == nil && len(raw) >= 32 {
			s.secret = raw
			return nil
		}
	}
	raw := make([]byte, 48)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("生成会话密钥失败: %w", err)
	}
	s.secret = raw
	return s.st.SetSetting("session_secret", base64.StdEncoding.EncodeToString(raw))
}

// Handler 返回带通用中间件的 http.Handler。
func (s *Server) Handler() http.Handler {
	return s.withRecover(s.withSecurityHeaders(s.withLogging(s.mux)))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/me", s.handleMe)
	s.mux.HandleFunc("POST /api/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("GET /api/tags", s.handleTags)
	s.mux.HandleFunc("GET /api/prompts", s.handleListPrompts)
	s.mux.HandleFunc("POST /api/prompts", s.requireAuth(s.handleCreatePrompt))
	s.mux.HandleFunc("GET /api/prompts/{id}", s.handleGetPrompt)
	s.mux.HandleFunc("PUT /api/prompts/{id}", s.requireAuth(s.handleUpdatePrompt))
	s.mux.HandleFunc("DELETE /api/prompts/{id}", s.requireAuth(s.handleDeletePrompt))
	s.mux.HandleFunc("POST /api/prompts/{id}/favorite", s.requireAuth(s.handleFavoritePrompt))
	s.mux.HandleFunc("POST /api/prompts/{id}/enabled", s.requireAuth(s.handleEnabledPrompt))
	s.mux.HandleFunc("GET /api/export", s.requireAuth(s.handleExport))
	s.mux.HandleFunc("/", s.handleStatic)
}

// ---------- 响应工具 ----------

type apiEnvelope struct {
	OK    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("写响应失败: %v", err)
	}
}

func writeOK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, apiEnvelope{OK: true, Data: data})
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, apiEnvelope{OK: false, Error: msg})
}

func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return false
	}
	return true
}

// ---------- 会话 / 鉴权 ----------

func (s *Server) issueToken() string {
	exp := time.Now().Add(sessionTTL).Unix()
	payload := strconv.FormatInt(s.epoch, 10) + "|" + strconv.FormatInt(exp, 10)
	return payload + "." + base64.RawURLEncoding.EncodeToString(s.sign([]byte(payload)))
}

func (s *Server) sign(payload []byte) []byte {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write(payload)
	return mac.Sum(nil)
}

func (s *Server) verifyToken(token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	idx := strings.LastIndex(token, ".")
	if idx <= 0 {
		return false
	}
	payload := token[:idx]
	sig, err := base64.RawURLEncoding.DecodeString(token[idx+1:])
	if err != nil {
		return false
	}
	if subtle.ConstantTimeCompare(sig, s.sign([]byte(payload))) != 1 {
		return false
	}
	parts := strings.Split(payload, "|")
	if len(parts) != 2 {
		return false
	}
	epoch, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || epoch != s.epoch {
		return false
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return false
	}
	return true
}

// tokenFromRequest 支持 Cookie 与 Authorization: Bearer 两种方式。
func tokenFromRequest(r *http.Request) string {
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return ""
}

func (s *Server) authenticated(r *http.Request) bool {
	return s.verifyToken(tokenFromRequest(r))
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authenticated(r) {
			writeErr(w, http.StatusUnauthorized, "需要管理员登录")
			return
		}
		next(w, r)
	}
}

func (s *Server) requestIsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return true
	}
	return false
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    s.issueToken(),
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   s.requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// ---------- 密码 ----------

// EnsureAdminPassword 确保存在管理员密码；首次返回新生成的明文密码。
func EnsureAdminPassword(st *store.Store) (password string, generated bool, err error) {
	v, ok, err := st.Setting("admin_password_hash")
	if err != nil {
		return "", false, err
	}
	if ok && v != "" {
		return "", false, nil
	}
	password, err = RandomPassword(16)
	if err != nil {
		return "", false, err
	}
	if err := SetAdminPassword(st, password); err != nil {
		return "", false, err
	}
	return password, true, nil
}

// SetAdminPassword 写入新的管理员密码并让旧会话失效。
func SetAdminPassword(st *store.Store, password string) error {
	password = strings.TrimSpace(password)
	if len([]rune(password)) < 8 {
		return errors.New("密码至少 8 位")
	}
	encoded, err := hashPassword(password)
	if err != nil {
		return err
	}
	if err := st.SetSetting("admin_password_hash", encoded); err != nil {
		return err
	}
	_, err = st.BumpAuthEpoch()
	return err
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, pbkdf2KeyLen)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", pbkdf2Iterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

func verifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter <= 0 || iter > 2000000 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

// RandomPassword 生成随机密码，去掉了容易看错的字符。
func RandomPassword(n int) (string, error) {
	const alphabet = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i, b := range buf {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(out), nil
}

// ---------- 登录限流 ----------

type rateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rlEntry
}

type rlEntry struct {
	fails    int
	lockedTo time.Time
	seen     time.Time
}

func newRateLimiter() *rateLimiter {
	l := &rateLimiter{entries: make(map[string]*rlEntry)}
	go l.gc()
	return l
}

func (l *rateLimiter) gc() {
	for range time.Tick(10 * time.Minute) {
		cutoff := time.Now().Add(-1 * time.Hour)
		l.mu.Lock()
		for k, e := range l.entries {
			if e.seen.Before(cutoff) {
				delete(l.entries, k)
			}
		}
		l.mu.Unlock()
	}
}

// allow 判断该来源当前是否允许尝试登录。
func (l *rateLimiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[key]
	if !ok {
		return true, 0
	}
	if time.Now().Before(e.lockedTo) {
		return false, time.Until(e.lockedTo)
	}
	return true, 0
}

func (l *rateLimiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[key]
	if !ok {
		e = &rlEntry{}
		l.entries[key] = e
	}
	e.fails++
	e.seen = time.Now()
	if e.fails >= 6 {
		e.lockedTo = time.Now().Add(15 * time.Minute)
		e.fails = 0
	}
}

func (l *rateLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

// ClientIP 取出真实来源 IP（nginx 会写 X-Real-IP）。
func ClientIP(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-Real-IP")); v != "" {
		return v
	}
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if idx := strings.Index(v, ","); idx > 0 {
			return strings.TrimSpace(v[:idx])
		}
		return strings.TrimSpace(v)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ---------- 中间件 ----------

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
		}
	})
}

func (s *Server) withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v (%s %s)", rec, r.Method, r.URL.Path)
				writeErr(w, http.StatusInternalServerError, "服务器内部错误")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
