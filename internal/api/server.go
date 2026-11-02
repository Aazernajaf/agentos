// Package api exposes read-only snapshots on one authenticated local origin.
package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/Aazernajaf/agentos/internal/fleet"
	"github.com/Aazernajaf/agentos/internal/projecttask"
)

type Server struct {
	store      *fleet.Store
	host       string
	token      string
	cookieName string
	files      http.Handler
	web        fs.FS
	task       *projecttask.Task
	reviewer   *projecttask.Reviewer
	csrf       string
}

// EnableReview is host configuration, never an HTTP/agent registration path.
// Call only before serving requests, with a live verified task held in memory.
func (s *Server) EnableReview(task *projecttask.Task, reviewer *projecttask.Reviewer) error {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return err
	}
	s.task = task
	s.reviewer = reviewer
	s.csrf = hex.EncodeToString(secret)
	return nil
}

func New(store *fleet.Store, host string, web fs.FS) (*Server, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	scope := sha256.Sum256([]byte(host))
	return &Server{store: store, host: host, token: hex.EncodeToString(secret), cookieName: "agentos_session_" + hex.EncodeToString(scope[:8]), files: http.FileServer(http.FS(web)), web: web}, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	origin := "http://" + s.host
	site := r.Header.Get("Sec-Fetch-Site")
	if r.Host != s.host || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != origin) || (site != "" && site != "none" && site != "same-origin") {
		http.Error(w, "Local origin required", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet && !(r.Method == http.MethodPost && r.URL.Path == "/v0/review/approve" && s.reviewer != nil) {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Read-only service", http.StatusMethodNotAllowed)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/v0/") {
		cookie, err := r.Cookie(s.cookieName)
		if err != nil || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(s.token)) != 1 {
			http.Error(w, "Open the local application first", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/v0/review", "/v0/review/approve":
			s.review(w, r)
		case "/v0/fleet":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(s.store.Snapshot())
		case "/v0/events":
			s.events(w, r)
		default:
			http.NotFound(w, r)
		}
		return
	}
	if r.URL.Path == "/" {
		// Only direct/same-origin navigation can bootstrap the local browser session.
		// This is not a security boundary against another process under the same OS user.
		http.SetCookie(w, &http.Cookie{Name: s.cookieName, Value: s.token, Path: "/v0/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
	}
	if r.URL.Path != "/" {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if !strings.HasPrefix(name, "assets/") || !fs.ValidPath(name) {
			http.NotFound(w, r)
			return
		}
		for _, part := range strings.Split(name, "/") {
			if strings.HasPrefix(part, ".") {
				http.NotFound(w, r)
				return
			}
		}
		info, err := fs.Stat(s.web, name)
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
	}
	s.files.ServeHTTP(w, r)
}

func (s *Server) review(w http.ResponseWriter, r *http.Request) {
	if s.task == nil || s.reviewer == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if r.URL.Path == "/v0/review" {
		_ = json.NewEncoder(w).Encode(struct {
			Report projecttask.Report `json:"report"`
			CSRF   string             `json:"csrf"`
		}{s.task.Report(), s.csrf})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", 405)
		return
	}
	if r.Header.Get("Origin") != "http://"+s.host || r.Header.Get("Sec-Fetch-Site") != "same-origin" || r.Header.Get("Content-Type") != "application/json" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-AgentOS-CSRF")), []byte(s.csrf)) != 1 {
		http.Error(w, "Operator origin required", 403)
		return
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	first, err := d.Token()
	if err != nil || first != json.Delim('{') {
		http.Error(w, "Invalid approval", 400)
		return
	}
	key, err := d.Token()
	if err != nil || key != "review_id" {
		http.Error(w, "Invalid approval", 400)
		return
	}
	var id string
	if d.Decode(&id) != nil {
		http.Error(w, "Invalid approval", 400)
		return
	}
	end, err := d.Token()
	if err != nil || end != json.Delim('}') {
		http.Error(w, "Invalid approval", 400)
		return
	}
	if _, err := d.Token(); err != io.EOF {
		http.Error(w, "Invalid approval", 400)
		return
	}
	if err := s.reviewer.Approve(id); err != nil {
		http.Error(w, "Evidence changed, expired or approval already used", 409)
		return
	}
	_ = json.NewEncoder(w).Encode(s.task.Report())
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unavailable", 500)
		return
	}
	ch, cancel, err := s.store.Watch()
	if err != nil {
		http.Error(w, "Too many subscribers", 503)
		return
	}
	defer cancel()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	send := func() bool {
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(5 * time.Second))
		snap := s.store.Snapshot()
		data, err := json.Marshal(snap)
		if err != nil {
			return false
		}
		if _, err = fmt.Fprintf(w, "id: %d\nevent: fleet\ndata: %s\n\n", snap.Cursor, data); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	if !send() {
		return
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			if !send() {
				return
			}
		case <-ticker.C:
			_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(5 * time.Second))
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
