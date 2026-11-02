package api

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Aazernajaf/agentos/internal/fleet"
)

func testServer(t *testing.T) (*Server, *fleet.Store) {
	t.Helper()
	store := fleet.New("demo")
	server, err := New(store, "127.0.0.1:8080", fstest.MapFS{"index.html": {Data: []byte("synthetic page")}})
	if err != nil {
		t.Fatal(err)
	}
	return server, store
}
func request(s *Server, path string, cookie *http.Cookie, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080"+path, nil)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func TestLocalBootstrapAndAuthorization(t *testing.T) {
	s, _ := testServer(t)
	if request(s, "/v0/fleet", nil, nil).Code != 401 {
		t.Fatal("unauthenticated API exposed")
	}
	page := request(s, "/", nil, nil)
	cookies := page.Result().Cookies()
	if page.Code != 200 || len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("missing protected local session")
	}
	if request(s, "/v0/fleet", cookies[0], nil).Code != 200 {
		t.Fatal("valid session rejected")
	}
	for _, headers := range []map[string]string{{"Origin": "https://evil.example"}, {"Sec-Fetch-Site": "cross-site"}, {"Sec-Fetch-Site": "same-site"}} {
		if request(s, "/v0/fleet", cookies[0], headers).Code != 403 {
			t.Fatal("cross-origin request accepted")
		}
	}
	r := httptest.NewRequest("GET", "http://evil.example/", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("forged host accepted")
	}
	r = httptest.NewRequest("POST", "http://127.0.0.1:8080/v0/fleet", strings.NewReader(`{}`))
	r.AddCookie(cookies[0])
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 405 {
		t.Fatal("mutation allowed")
	}
}
func TestSSEReconnectReturnsFullCurrentSnapshot(t *testing.T) {
	s, store := testServer(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { r.Host = s.host; s.ServeHTTP(w, r) }))
	defer ts.Close()
	ev := fleet.Event{Identity: fleet.Identity{Provider: "codex", Version: "test", Installation: "test", Workspace: "test", Session: "owned"}, Sequence: 1, Turn: "turn-1", State: "running", At: time.Now()}
	_, _ = store.Apply(ev)
	for _, cursor := range []string{"", "999999"} {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/v0/events", nil)
		req.AddCookie(&http.Cookie{Name: s.cookieName, Value: s.token})
		req.Header.Set("Last-Event-ID", cursor)
		response, err := ts.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		reader := bufio.NewReader(response.Body)
		var frame strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			frame.WriteString(line)
			if line == "\n" {
				break
			}
		}
		if !strings.Contains(frame.String(), `"execution_state":"running"`) || !strings.Contains(frame.String(), "event: fleet") {
			t.Fatal("missing full snapshot")
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(reader, 0))
		response.Body.Close()
		cancel()
	}
}

func TestStaticBoundaryRejectsListingsAndHiddenFiles(t *testing.T) {
	s, _ := testServer(t)
	for _, path := range []string{"/assets/", "/.env", "/assets/._private", "/unknown", "/assets/../index.html"} {
		if request(s, path, nil, nil).Code != 404 {
			t.Fatalf("static path exposed: %s", path)
		}
	}
}

func TestParallelLocalInstancesHaveDistinctCookieNames(t *testing.T) {
	first, store := testServer(t)
	second, err := New(store, "127.0.0.1:8081", fstest.MapFS{"index.html": {Data: []byte("synthetic")}})
	if err != nil {
		t.Fatal(err)
	}
	if first.cookieName == second.cookieName {
		t.Fatal("parallel origins would overwrite each other's session cookie")
	}
}
