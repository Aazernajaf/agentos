package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Aazernajaf/agentos/internal/projecttask"
)

func reviewFixture(t *testing.T) (*Server, string, string) {
	t.Helper()
	dir := t.TempDir()
	data := []byte("reviewed result")
	os.WriteFile(filepath.Join(dir, "result.txt"), data, 0600)
	sum := sha256.Sum256(data)
	task, reviewer, err := projecttask.New(dir, "../..", projecttask.Manifest{TaskID: "real-task", SessionID: "owned", Inputs: []string{"result.txt"}, Artifacts: []projecttask.Artifact{{Path: "result.txt", SHA256: hex.EncodeToString(sum[:])}}, Tests: []projecttask.Test{{ID: "file-test", Argv: []string{"/bin/sh", "-c", "test -f result.txt"}}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { task.Close() })
	report, err := task.Verify(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	s, _ := testServer(t)
	if err := s.EnableReview(task, reviewer); err != nil {
		t.Fatal(err)
	}
	return s, report.ReviewID, dir
}
func approveRequest(s *Server, id, origin, csrf, body string, cookie bool) *httptest.ResponseRecorder {
	if body == "" {
		body = `{"review_id":"` + id + `"}`
	}
	r := httptest.NewRequest("POST", "http://"+s.host+"/v0/review/approve", strings.NewReader(body))
	r.Header.Set("Origin", origin)
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-AgentOS-CSRF", csrf)
	if cookie {
		r.AddCookie(&http.Cookie{Name: s.cookieName, Value: s.token})
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func TestReviewRequiresOperatorOriginCookieAndCSRF(t *testing.T) {
	s, id, _ := reviewFixture(t)
	origin := "http://" + s.host
	for _, tc := range []struct {
		origin, csrf, body string
		cookie             bool
		status             int
	}{
		{origin, s.csrf, "", false, 401}, {"", s.csrf, "", true, 403}, {"https://evil.example", s.csrf, "", true, 403}, {origin, "wrong", "", true, 403},
		{origin, s.csrf, `{"review_id":"x","review_id":"y"}`, true, 400}, {origin, s.csrf, `{"review_id":"x","approved":true}`, true, 400},
	} {
		if w := approveRequest(s, id, tc.origin, tc.csrf, tc.body, tc.cookie); w.Code != tc.status {
			t.Fatalf("got %d want %d", w.Code, tc.status)
		}
	}
	if s.task.Report().State != "awaiting_user_approval" {
		t.Fatal("negative requests changed acceptance")
	}
	if w := approveRequest(s, id, origin, s.csrf, "", true); w.Code != 200 {
		t.Fatal("approved request failed", w.Code)
	}
	if s.task.Report().State != "accepted" {
		t.Fatal("operator action did not accept")
	}
	if w := approveRequest(s, id, origin, s.csrf, "", true); w.Code != 409 {
		t.Fatal("approval replay allowed")
	}
}
func TestStaleFilesCannotBeApprovedFromUI(t *testing.T) {
	s, id, dir := reviewFixture(t)
	os.WriteFile(filepath.Join(dir, "result.txt"), []byte("changed"), 0600)
	if s.task.Report().State != "stale" {
		t.Fatal("stale evidence still enabled")
	}
	if w := approveRequest(s, id, "http://"+s.host, s.csrf, "", true); w.Code != 409 {
		t.Fatal("stale content accepted")
	}
}
