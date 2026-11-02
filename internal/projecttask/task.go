// Package projecttask binds real workspace artifacts and host-selected test
// processes to governance receipts. It does not sandbox project code.
package projecttask

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/Aazernajaf/agentos/internal/governance"
)

var ErrEvidence = errors.New("project_evidence_invalid")

// Manifest is trusted operator configuration, never a provider request.
// Inputs must include every meaningful source, test and configuration input.
type Manifest struct {
	TaskID    string     `json:"task_id"`
	SessionID string     `json:"session_id"`
	Inputs    []string   `json:"inputs"`
	Artifacts []Artifact `json:"artifacts"`
	Tests     []Test     `json:"tests"`
}
type Artifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type Test struct {
	ID   string   `json:"id"`
	Argv []string `json:"argv"`
}
type TestEvidence struct {
	ID           string `json:"id"`
	Passed       bool   `json:"passed"`
	OutputSHA256 string `json:"output_sha256"`
	OutputBytes  int    `json:"output_bytes"`
}
type Report struct {
	Previews  []Preview      `json:"previews"`
	TaskID    string         `json:"task_id"`
	ExpiresAt time.Time      `json:"expires_at"`
	ReviewID  string         `json:"review_id"`
	Revision  string         `json:"revision"`
	Artifacts []Artifact     `json:"artifacts"`
	Tests     []TestEvidence `json:"tests"`
	State     string         `json:"state"`
}
type Preview struct {
	Path      string `json:"path"`
	Text      string `json:"text"`
	Truncated bool   `json:"truncated"`
}
type Task struct {
	runtime  *governance.Runtime
	mu       sync.Mutex
	manifest Manifest
	root     *os.Root
	report   Report
	proof    string
}

// Reviewer is host-only authority. Never hand it to a provider or agent tool.
type Reviewer struct {
	task     *Task
	operator *governance.Operator
}

func safePath(s string) bool {
	if s == "" || path.Clean(s) != s || s == "." || strings.ContainsAny(s, "\\\x00") || strings.HasPrefix(s, "/") {
		return false
	}
	for _, p := range strings.Split(s, "/") {
		if p == ".." || strings.EqualFold(p, ".git") {
			return false
		}
	}
	return true
}
func digest(data []byte) string { s := sha256.Sum256(data); return hex.EncodeToString(s[:]) }

// New reads the current real files; no tests or model processes start here.
// Caller must keep the returned pair alive for the task's whole budget.
func New(workspace, policyRoot string, m Manifest) (*Task, *Reviewer, error) {
	// Deep copy host configuration so later caller mutations cannot alter checks.
	encoded, err := json.Marshal(m)
	if err != nil {
		return nil, nil, ErrEvidence
	}
	m = Manifest{}
	if json.Unmarshal(encoded, &m) != nil {
		return nil, nil, ErrEvidence
	}
	if len(m.Inputs) == 0 || len(m.Inputs) > 64 || len(m.Artifacts) == 0 || len(m.Artifacts) > 64 || len(m.Tests) == 0 || len(m.Tests) > 32 {
		return nil, nil, ErrEvidence
	}
	seen := map[string]bool{}
	for _, p := range m.Inputs {
		if !safePath(p) || seen[p] {
			return nil, nil, ErrEvidence
		}
		seen[p] = true
	}
	seen = map[string]bool{}
	for _, a := range m.Artifacts {
		b, err := hex.DecodeString(a.SHA256)
		if !safePath(a.Path) || seen[a.Path] || err != nil || len(b) != 32 || strings.ToLower(a.SHA256) != a.SHA256 {
			return nil, nil, ErrEvidence
		}
		seen[a.Path] = true
	}
	seen = map[string]bool{}
	for _, test := range m.Tests {
		if !safePath(test.ID) || seen[test.ID] || len(test.Argv) == 0 || len(test.Argv) > 64 {
			return nil, nil, ErrEvidence
		}
		seen[test.ID] = true
		for _, a := range test.Argv {
			if len(a) > 4096 || strings.ContainsRune(a, 0) {
				return nil, nil, ErrEvidence
			}
		}
		if test.Argv[0] == "" {
			return nil, nil, ErrEvidence
		}
	}
	root, err := os.OpenRoot(workspace)
	if err != nil {
		return nil, nil, ErrEvidence
	}
	t := &Task{root: root, manifest: m}
	fail := func(err error) (*Task, *Reviewer, error) { root.Close(); return nil, nil, err }
	revision, err := t.revision()
	if err != nil {
		return fail(err)
	}
	skill, err := governance.LoadSkill(policyRoot, "agentos-implementation")
	if err != nil {
		return fail(err)
	}
	scopes := append([]string{}, m.Inputs...)
	for _, a := range m.Artifacts {
		scopes = append(scopes, a.Path)
	}
	// A fixed read operation connects task admission to the same project scope.
	resources := []string{m.Artifacts[0].Path}
	runtime, operator, err := governance.New(governance.Config{
		Spec:            governance.Spec{TaskID: m.TaskID, SessionID: m.SessionID, Revision: revision, Skill: skill, Scope: scopes, AllowedOperations: []string{"inspect-artifacts"}, RequiredChecks: []string{"artifacts-and-tests"}, MaxActions: 1, MaxVerifications: 1, ExecutionTimeout: 2 * time.Minute, EvidenceTTL: 15 * time.Minute, AcceptanceMode: governance.OperatorReview},
		CurrentRevision: t.revision, CurrentSkill: func() (governance.Skill, error) { return governance.LoadSkill(policyRoot, skill.ID) },
		Operations: map[string]governance.Operation{"inspect-artifacts": {Kind: governance.Read, Resources: resources, OncePerRevision: true, Run: func(context.Context) error { return t.artifacts() }}},
		Checks:     map[string]func(context.Context) error{"artifacts-and-tests": t.check},
	})
	if err != nil {
		return fail(err)
	}
	t.runtime = runtime
	return t, &Reviewer{t, operator}, nil
}
func (t *Task) Close() error { return t.root.Close() }

// revision hashes declared input trees, output bytes and the complete manifest.
// Walks reject symlinks/special files and cap the entire selected tree at 64 MiB.
func (t *Task) revision() (string, error) {
	h := sha256.New()
	b, _ := json.Marshal(t.manifest)
	h.Write(b)
	files := map[string]bool{}
	entries := 0
	total := int64(0)
	paths := append([]string{}, t.manifest.Inputs...)
	for _, a := range t.manifest.Artifacts {
		paths = append(paths, a.Path)
	}
	for _, p := range paths {
		// Reject symlinks in parent components as well as walked entries.
		parts := strings.Split(p, "/")
		for i := range parts {
			info, err := t.root.Lstat(strings.Join(parts[:i+1], "/"))
			if err != nil || info.Mode()&os.ModeSymlink != 0 {
				return "", ErrEvidence
			}
		}
		err := fs.WalkDir(t.root.FS(), p, func(name string, d fs.DirEntry, err error) error {
			if err != nil || !safePath(name) || d.Type()&os.ModeSymlink != 0 {
				return ErrEvidence
			}
			if files[name] {
				return nil
			}
			files[name] = true
			entries++
			if entries > 4096 {
				return ErrEvidence
			}
			info, err := d.Info()
			if err != nil || (!info.Mode().IsRegular() && !info.IsDir()) {
				return ErrEvidence
			}
			metadata, _ := json.Marshal(struct {
				Path string
				Mode uint32
			}{name, uint32(info.Mode())})
			h.Write(metadata)
			if info.IsDir() {
				return nil
			}
			total += info.Size()
			if total > 64<<20 {
				return ErrEvidence
			}
			f, err := t.root.Open(name)
			if err != nil {
				return ErrEvidence
			}
			content, err := io.ReadAll(io.LimitReader(f, 64<<20+1))
			f.Close()
			if err != nil || len(content) > 64<<20 {
				return ErrEvidence
			}
			// Length-framed names/content avoid ambiguous concatenations.
			record, _ := json.Marshal(Artifact{Path: name, SHA256: digest(content)})
			h.Write(record)
			return nil
		})
		if err != nil {
			return "", ErrEvidence
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func (t *Task) artifacts() error {
	for _, a := range t.manifest.Artifacts {
		f, err := t.root.Open(a.Path)
		if err != nil {
			return ErrEvidence
		}
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 64<<20 {
			f.Close()
			return ErrEvidence
		}
		h := sha256.New()
		n, err := io.Copy(h, io.LimitReader(f, 64<<20+1))
		f.Close()
		if err != nil || n > 64<<20 || hex.EncodeToString(h.Sum(nil)) != a.SHA256 {
			return ErrEvidence
		}
	}
	return nil
}

type output struct {
	hash io.Writer
	n    int
}

func (o *output) Write(p []byte) (int, error) {
	if o.n+len(p) > 1<<20 {
		return 0, ErrEvidence
	}
	o.n += len(p)
	return o.hash.Write(p)
}

func (t *Task) check(ctx context.Context) error {
	if err := t.artifacts(); err != nil {
		return err
	}
	var results []TestEvidence
	for _, test := range t.manifest.Tests {
		cmd := exec.CommandContext(ctx, test.Argv[0], test.Argv[1:]...)
		cmd.Dir = t.root.Name()
		cmd.WaitDelay = time.Second
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error {
			if cmd.Process == nil {
				return nil
			}
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		// Deliberately exclude provider credentials, inherited session bridges and tokens.
		for _, k := range []string{"PATH", "HOME", "TMPDIR", "LANG", "LC_ALL"} {
			if v, ok := os.LookupEnv(k); ok {
				cmd.Env = append(cmd.Env, k+"="+v)
			}
		}
		cmd.Env = append(cmd.Env, "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off")
		h := sha256.New()
		out := &output{hash: h}
		cmd.Stdout = out
		cmd.Stderr = out
		err := cmd.Run()
		results = append(results, TestEvidence{test.ID, err == nil && ctx.Err() == nil, hex.EncodeToString(h.Sum(nil)), out.n})
		t.mu.Lock()
		t.report.Tests = append([]TestEvidence{}, results...)
		t.mu.Unlock()
		if err != nil || ctx.Err() != nil {
			return ErrEvidence
		}
	}
	return t.artifacts()
}

// Verify never calls Review or Accept. The report is pending human approval.
func (t *Task) Verify(ctx context.Context) (Report, error) {
	started := time.Now()
	spec := t.runtime.Contract()
	req := governance.Request{ID: "inspect", TaskID: spec.TaskID, SessionID: spec.SessionID, Revision: spec.Revision, Skill: spec.Skill, Operation: "inspect-artifacts"}
	if err := t.runtime.Dispatch(ctx, req); err != nil {
		return Report{}, err
	}
	if err := t.runtime.Submit(spec.TaskID, spec.SessionID); err != nil {
		return Report{}, err
	}
	proof, err := t.runtime.Verify(ctx, spec.TaskID, spec.SessionID)
	if err != nil {
		return Report{}, err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return Report{}, ErrEvidence
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.proof = proof
	t.report.ReviewID = hex.EncodeToString(nonce[:])
	t.report.Revision = spec.Revision
	t.report.Artifacts = append([]Artifact{}, t.manifest.Artifacts...)
	for _, a := range t.manifest.Artifacts {
		f, err := t.root.Open(a.Path)
		if err != nil {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(f, 8193))
		f.Close()
		if err != nil {
			continue
		}
		truncated := len(data) > 8192
		if truncated {
			data = data[:8192]
			for len(data) > 0 && !utf8.Valid(data) {
				data = data[:len(data)-1]
			}
		}
		if utf8.Valid(data) && !strings.ContainsRune(string(data), 0) {
			t.report.Previews = append(t.report.Previews, Preview{a.Path, string(data), truncated})
		}
	}
	t.report.State = "awaiting_user_approval"
	t.report.TaskID = spec.TaskID
	t.report.ExpiresAt = started.Add(15 * time.Minute)
	return clone(t.report), nil
}
func clone(r Report) Report {
	r.Previews = append([]Preview{}, r.Previews...)
	r.Artifacts = append([]Artifact{}, r.Artifacts...)
	r.Tests = append([]TestEvidence{}, r.Tests...)
	return r
}
func (t *Task) Report() Report {
	t.mu.Lock()
	defer t.mu.Unlock()
	r := clone(t.report)
	if r.State == "awaiting_user_approval" {
		current, err := t.revision()
		if err != nil || current != r.Revision || !time.Now().Before(r.ExpiresAt) {
			r.State = "stale"
		}
	}
	if r.State == "" {
		r.State = t.runtime.Snapshot().State
	}
	return r
}

// Approve is called only by the trusted operator UI after a specific user action.
// The caller must authenticate that action; a review ID is not authentication.
func (r *Reviewer) Approve(reviewID string) error {
	t := r.task
	t.mu.Lock()
	defer t.mu.Unlock()
	if reviewID == "" || reviewID != t.report.ReviewID || t.proof == "" {
		return governance.ErrReview
	}
	if err := r.operator.Review(t.proof); err != nil {
		return err
	}
	spec := t.runtime.Contract()
	if err := t.runtime.Accept(spec.TaskID, spec.SessionID, t.proof); err != nil {
		return err
	}
	t.proof = ""
	t.report.State = "accepted"
	return nil
}

// DecodeManifest is bounded and rejects unknown fields and appended JSON.
// Load it only from trusted operator configuration, never an agent output file.
func DecodeManifest(r io.Reader) (Manifest, error) {
	data, err := io.ReadAll(io.LimitReader(r, 65537))
	if err != nil || len(data) > 65536 {
		return Manifest{}, ErrEvidence
	}
	d := json.NewDecoder(strings.NewReader(string(data)))
	if strictValue(d) != nil {
		return Manifest{}, ErrEvidence
	}
	if _, err := d.Token(); err != io.EOF {
		return Manifest{}, ErrEvidence
	}
	d = json.NewDecoder(strings.NewReader(string(data)))
	d.DisallowUnknownFields()
	var m Manifest
	if d.Decode(&m) != nil {
		return Manifest{}, ErrEvidence
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return Manifest{}, ErrEvidence
	}
	return m, nil
}

func strictValue(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return ErrEvidence
	}
	switch token {
	case json.Delim('{'):
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return ErrEvidence
			}
			s, ok := key.(string)
			if !ok || seen[s] {
				return ErrEvidence
			}
			seen[s] = true
			switch s {
			case "task_id", "session_id", "inputs", "artifacts", "tests", "path", "sha256", "id", "argv":
			default:
				return ErrEvidence
			}
			if strictValue(d) != nil {
				return ErrEvidence
			}
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return ErrEvidence
		}
	case json.Delim('['):
		for d.More() {
			if strictValue(d) != nil {
				return ErrEvidence
			}
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return ErrEvidence
		}
	case json.Delim('}'), json.Delim(']'):
		return ErrEvidence
	}
	return nil
}
