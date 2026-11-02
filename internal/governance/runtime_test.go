package governance

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fixture struct {
	root       string
	runtime    *Runtime
	operator   *Operator
	spec       Spec
	revision   atomic.Value
	now        atomic.Value
	effects    atomic.Int32
	checkCalls atomic.Int32
	fail       atomic.Bool
}

func setup(t *testing.T, change func(*Config)) *fixture {
	t.Helper()
	f := &fixture{root: t.TempDir()}
	dir := filepath.Join(f.root, ".agents", "skills", "test-skill")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SKILL.md", "SOP.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("Synthetic "+name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(f.root, "AGENTS.md"), []byte("Synthetic root rules"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(f.root, "docs/agents"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.root, "docs/agents/operating-policy.md"), []byte("Synthetic policy"), 0600); err != nil {
		t.Fatal(err)
	}
	skill, err := LoadSkill(f.root, "test-skill")
	if err != nil {
		t.Fatal(err)
	}
	f.revision.Store("revision-1")
	f.now.Store(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC))
	cfg := Config{
		Spec:            Spec{TaskID: "task-1", SessionID: "owned-session", Revision: "revision-1", Skill: skill, Scope: []string{"internal", "migrations", "go.mod", ".agents"}, AllowedOperations: []string{"edit", "migration", "dependencies", "outside", "control", "policy"}, RequiredChecks: []string{"tests", "criteria"}, MaxActions: 4, MaxVerifications: 3, ExecutionTimeout: time.Second, EvidenceTTL: time.Minute, AcceptanceMode: OperatorReview},
		CurrentRevision: func() (string, error) { return f.revision.Load().(string), nil },
		CurrentSkill:    func() (Skill, error) { return LoadSkill(f.root, "test-skill") },
		Operations:      map[string]Operation{}, Checks: map[string]func(context.Context) error{},
		Now: func() time.Time { return f.now.Load().(time.Time) },
	}
	handler := func(context.Context) error { f.effects.Add(1); return nil }
	cfg.Operations["edit"] = Operation{Kind: Write, Resources: []string{"internal/result.go"}, Run: handler}
	cfg.Operations["migration"] = Operation{Kind: Write, Resources: []string{"migrations/synthetic.sql"}, Run: handler}
	cfg.Operations["dependencies"] = Operation{Kind: Write, Resources: []string{"go.mod"}, Run: handler}
	cfg.Operations["outside"] = Operation{Kind: Write, Resources: []string{"unrelated/file.go"}, Run: handler}
	cfg.Operations["control"] = Operation{Kind: ProviderControl, Resources: []string{"internal/owned-session"}, Run: handler}
	cfg.Operations["policy"] = Operation{Kind: Write, Resources: []string{".agents"}, Run: handler}
	cfg.Checks["tests"] = func(context.Context) error {
		f.checkCalls.Add(1)
		if f.fail.Load() {
			return errors.New("SECRET_CANARY")
		}
		return nil
	}
	cfg.Checks["criteria"] = func(context.Context) error {
		f.checkCalls.Add(1)
		if f.effects.Load() == 0 {
			return errors.New("outcome missing")
		}
		return nil
	}
	if change != nil {
		change(&cfg)
	}
	f.spec = cfg.Spec
	f.runtime, f.operator, err = New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func (f *fixture) request(id, operation string) Request {
	return Request{ID: id, TaskID: f.spec.TaskID, SessionID: f.spec.SessionID, Revision: f.spec.Revision, Skill: f.spec.Skill, Operation: operation}
}
func (f *fixture) verify(t *testing.T) string {
	t.Helper()
	if err := f.runtime.Submit(f.spec.TaskID, f.spec.SessionID); err != nil {
		t.Fatal(err)
	}
	token, err := f.runtime.Verify(context.Background(), f.spec.TaskID, f.spec.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	return token
}
func (f *fixture) accept(token string) error {
	return f.runtime.Accept(f.spec.TaskID, f.spec.SessionID, token)
}

func TestDeniedRequestsNeverReachExecutor(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Request)
		want   error
	}{
		{"foreign task", func(r *Request) { r.TaskID = "other" }, ErrOwner},
		{"foreign session", func(r *Request) { r.SessionID = "other" }, ErrOwner},
		{"stale revision", func(r *Request) { r.Revision = "old" }, ErrRevision},
		{"wrong SOP", func(r *Request) { r.Skill.SOPHash = strings.Repeat("a", 64) }, ErrSkill},
		{"unknown operation", func(r *Request) { r.Operation = "unknown" }, ErrScope},
		{"outside scope", func(r *Request) { r.Operation = "outside" }, ErrScope},
		{"migration disguised as write", func(r *Request) { r.Operation = "migration" }, ErrApproval},
		{"dependency disguised as write", func(r *Request) { r.Operation = "dependencies" }, ErrApproval},
		{"policy directory write", func(r *Request) { r.Operation = "policy" }, ErrApproval},
		{"unverified capability", func(r *Request) { r.Operation = "control" }, ErrCapability},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			f := setup(t, nil)
			req := f.request("request-1", "edit")
			test.mutate(&req)
			if err := f.runtime.Dispatch(context.Background(), req); err != test.want {
				t.Fatalf("got %v, want %v", err, test.want)
			}
			if f.effects.Load() != 0 {
				t.Fatal("denied operation executed")
			}
		})
	}
}
func TestExplicitApprovalIsBoundAndSingleUse(t *testing.T) {
	f := setup(t, nil)
	req := f.request("request-1", "migration")
	if f.operator.Approve(req, []Kind{Dependency}) != ErrApproval {
		t.Fatal("wrong category granted migration")
	}
	if err := f.operator.Approve(req, []Kind{Migration}); err != nil {
		t.Fatal(err)
	}
	changed := req
	changed.Operation = "dependencies"
	if f.runtime.Dispatch(context.Background(), changed) != ErrApproval {
		t.Fatal("approval transferred to different operation")
	}
	var wg sync.WaitGroup
	var successful atomic.Int32
	for range 8 {
		wg.Go(func() {
			if f.runtime.Dispatch(context.Background(), req) == nil {
				successful.Add(1)
			}
		})
	}
	wg.Wait()
	if successful.Load() != 1 || f.effects.Load() != 1 {
		t.Fatal("approval replayed")
	}
}
func TestAllProtectedCategoriesRequired(t *testing.T) {
	f := setup(t, func(c *Config) {
		op := c.Operations["migration"]
		op.Resources = append(op.Resources, "go.mod")
		c.Operations["migration"] = op
	})
	req := f.request("request-1", "migration")
	if f.operator.Approve(req, []Kind{Migration}) != ErrApproval {
		t.Fatal("partial approval accepted")
	}
	if err := f.operator.Approve(req, []Kind{Migration, Dependency}); err != nil {
		t.Fatal(err)
	}
	if err := f.runtime.Dispatch(context.Background(), req); err != nil {
		t.Fatal(err)
	}
}
func TestCompletionNeedsIssuedProofAndOperatorReview(t *testing.T) {
	f := setup(t, nil)
	if f.accept("claimed-done") != ErrProof {
		t.Fatal("agent completion claim accepted")
	}
	if err := f.runtime.Dispatch(context.Background(), f.request("request-1", "edit")); err != nil {
		t.Fatal(err)
	}
	token := f.verify(t)
	if f.accept("forged") != ErrProof {
		t.Fatal("forged proof accepted")
	}
	if f.accept(token) != ErrReview {
		t.Fatal("missing review accepted")
	}
	if err := f.operator.Review(token); err != nil {
		t.Fatal(err)
	}
	if err := f.accept(token); err != nil {
		t.Fatal(err)
	}
	if f.runtime.Snapshot().State != "accepted" {
		t.Fatal("valid result not accepted")
	}
	if f.runtime.Dispatch(context.Background(), f.request("request-2", "edit")) != ErrState {
		t.Fatal("accepted task changed")
	}
	if f.accept(token) != ErrState {
		t.Fatal("proof replay accepted")
	}
	data, _ := json.Marshal(f.runtime.Snapshot())
	if strings.Contains(string(data), token) {
		t.Fatal("proof leaked into snapshot")
	}
}
func TestFailedRequiredCheckIssuesNoProof(t *testing.T) {
	f := setup(t, nil)
	f.fail.Store(true)
	_ = f.runtime.Dispatch(context.Background(), f.request("request-1", "edit"))
	_ = f.runtime.Submit(f.spec.TaskID, f.spec.SessionID)
	token, err := f.runtime.Verify(context.Background(), f.spec.TaskID, f.spec.SessionID)
	if err != ErrCheck || token != "" || f.runtime.Snapshot().State != "active" {
		t.Fatal("failed check bypassed")
	}
	if f.accept("claim") != ErrProof {
		t.Fatal("failed evidence accepted")
	}
	data, _ := json.Marshal(f.runtime.Snapshot())
	if strings.Contains(string(data), "SECRET_CANARY") {
		t.Fatal("raw verifier error leaked")
	}
}
func TestActualSOPChangeInvalidatesApprovalAndProof(t *testing.T) {
	for _, phase := range []string{"approval", "proof"} {
		t.Run(phase, func(t *testing.T) {
			f := setup(t, nil)
			req := f.request("request-1", "migration")
			var token string
			if phase == "approval" {
				_ = f.operator.Approve(req, []Kind{Migration})
			} else {
				_ = f.runtime.Dispatch(context.Background(), f.request("edit-1", "edit"))
				token = f.verify(t)
				_ = f.operator.Review(token)
			}
			if err := os.WriteFile(filepath.Join(f.root, ".agents/skills/test-skill/SOP.md"), []byte("Changed SOP"), 0600); err != nil {
				t.Fatal(err)
			}
			if phase == "approval" {
				if f.runtime.Dispatch(context.Background(), req) != ErrSkill || f.effects.Load() != 0 {
					t.Fatal("stale SOP approval executed")
				}
			} else if f.accept(token) != ErrSkill {
				t.Fatal("stale SOP proof accepted")
			}
		})
	}
}
func TestRevisionChangeInvalidatesReviewedEvidence(t *testing.T) {
	f := setup(t, nil)
	_ = f.runtime.Dispatch(context.Background(), f.request("edit-1", "edit"))
	token := f.verify(t)
	_ = f.operator.Review(token)
	f.revision.Store("revision-2")
	if f.accept(token) != ErrRevision {
		t.Fatal("old revision evidence accepted")
	}
	if err := f.operator.Rebase("revision-2", f.spec.Skill); err != nil {
		t.Fatal(err)
	}
	if f.accept(token) != ErrProof {
		t.Fatal("rebase retained old proof")
	}
}
func TestExpiredAndClockRewoundProofRejected(t *testing.T) {
	for _, delta := range []time.Duration{time.Minute, -time.Second} {
		t.Run(delta.String(), func(t *testing.T) {
			f := setup(t, nil)
			_ = f.runtime.Dispatch(context.Background(), f.request("edit-1", "edit"))
			token := f.verify(t)
			_ = f.operator.Review(token)
			f.now.Store(f.now.Load().(time.Time).Add(delta))
			if f.accept(token) != ErrProof {
				t.Fatal("expired/clock-rewound proof accepted")
			}
		})
	}
}
func TestPauseCancelsInFlightWorkAndBlocksCompletion(t *testing.T) {
	started := make(chan struct{})
	f := setup(t, func(c *Config) {
		op := c.Operations["edit"]
		op.Run = func(ctx context.Context) error { close(started); <-ctx.Done(); return ctx.Err() }
		c.Operations["edit"] = op
	})
	finished := make(chan error, 1)
	go func() { finished <- f.runtime.Dispatch(context.Background(), f.request("edit-1", "edit")) }()
	<-started
	if f.runtime.Submit(f.spec.TaskID, f.spec.SessionID) != ErrBusy {
		t.Fatal("submitted while work running")
	}
	f.operator.Pause()
	if err := <-finished; err != ErrExecution {
		t.Fatal("paused work claimed success")
	}
	if f.runtime.Dispatch(context.Background(), f.request("edit-2", "edit")) != ErrState {
		t.Fatal("paused task executed")
	}
	if err := f.operator.Resume(); err != nil {
		t.Fatal(err)
	}
	if f.runtime.Snapshot().ActionsUsed != 1 {
		t.Fatal("resume reset action budget")
	}
	f.operator.Cancel()
	if f.operator.Resume() != ErrState {
		t.Fatal("cancelled task resumed")
	}
}
func TestPauseDuringVerificationCannotIssueProof(t *testing.T) {
	started := make(chan struct{})
	f := setup(t, func(c *Config) {
		c.Checks["tests"] = func(ctx context.Context) error { close(started); <-ctx.Done(); return ctx.Err() }
	})
	_ = f.runtime.Submit(f.spec.TaskID, f.spec.SessionID)
	result := make(chan error, 1)
	go func() {
		token, err := f.runtime.Verify(context.Background(), f.spec.TaskID, f.spec.SessionID)
		if token != "" {
			result <- errors.New("proof issued")
			return
		}
		result <- err
	}()
	<-started
	f.operator.Pause()
	if err := <-result; err != ErrState {
		t.Fatal("paused verification accepted", err)
	}
}
func TestRetryBudgetsSurviveRebaseAndResume(t *testing.T) {
	f := setup(t, func(c *Config) { c.Spec.MaxActions = 1; c.Spec.MaxVerifications = 1 })
	_ = f.runtime.Dispatch(context.Background(), f.request("edit-1", "edit"))
	f.operator.Pause()
	_ = f.operator.Resume()
	if err := f.operator.Rebase(f.spec.Revision, f.spec.Skill); err != nil {
		t.Fatal(err)
	}
	if f.runtime.Dispatch(context.Background(), f.request("edit-2", "edit")) != ErrBudget {
		t.Fatal("action budget reset")
	}
	f.fail.Store(true)
	_ = f.runtime.Submit(f.spec.TaskID, f.spec.SessionID)
	_, _ = f.runtime.Verify(context.Background(), f.spec.TaskID, f.spec.SessionID)
	_ = f.runtime.Submit(f.spec.TaskID, f.spec.SessionID)
	if _, err := f.runtime.Verify(context.Background(), f.spec.TaskID, f.spec.SessionID); err != ErrBudget {
		t.Fatal("verification retry budget reset")
	}
}
func TestVerifierDetectsInputsChangedWhileChecksRun(t *testing.T) {
	f := setup(t, nil)
	f.effects.Store(1)
	f.runtime.cfg.Checks["tests"] = func(context.Context) error { f.revision.Store("revision-2"); return nil }
	_ = f.runtime.Submit(f.spec.TaskID, f.spec.SessionID)
	if token, err := f.runtime.Verify(context.Background(), f.spec.TaskID, f.spec.SessionID); err != ErrRevision || token != "" {
		t.Fatal("changed inputs issued proof")
	}
}
func TestPanicAndTimeoutFailClosed(t *testing.T) {
	for _, mode := range []string{"panic", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			f := setup(t, func(c *Config) {
				c.Spec.ExecutionTimeout = 10 * time.Millisecond
				op := c.Operations["edit"]
				op.Run = func(ctx context.Context) error {
					if mode == "panic" {
						panic("SECRET_CANARY")
					}
					<-ctx.Done()
					return nil
				}
				c.Operations["edit"] = op
			})
			if f.runtime.Dispatch(context.Background(), f.request("edit-1", "edit")) != ErrExecution {
				t.Fatal("failed handler claimed success")
			}
			if f.runtime.Snapshot().ActionsUsed != 1 {
				t.Fatal("failed handler refunded retry budget")
			}
		})
	}
}
func TestRequestCannotImportApprovalsOrCheckResults(t *testing.T) {
	f := setup(t, nil)
	data, _ := json.Marshal(f.request("request-1", "edit"))
	if _, err := DecodeRequest(data); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{`,"approved":true}`, `,"check_results":{"tests":"passed"}}`, `,"command":"arbitrary shell"}`, `,"operation":"migration"}`, `,"ID":"different"}`} {
		forged := append(append([]byte(nil), data[:len(data)-1]...), []byte(suffix)...)
		if _, err := DecodeRequest(forged); err != ErrInvalid {
			t.Fatal("agent imported authority")
		}
	}
	for _, data := range [][]byte{[]byte(`null`), []byte(`{} {}`), []byte(strings.Repeat("x", 16*1024+1))} {
		if _, err := DecodeRequest(data); err != ErrInvalid {
			t.Fatal("invalid request accepted")
		}
	}
}
func TestContractRejectsVacuousChecksAndTraversal(t *testing.T) {
	f := setup(t, nil)
	config := f.runtime.cfg
	config.Spec.RequiredChecks = nil
	if _, _, err := New(config); err != ErrInvalid {
		t.Fatal("no-criteria contract accepted")
	}
	for _, resource := range []string{"../outside", "/absolute", "internal/../outside", ".git/config", "internal\\outside"} {
		if validResource(resource) {
			t.Fatal("unsafe logical resource accepted")
		}
	}
}
func TestSkillLoaderRejectsMissingOrEscapingCanonicalFiles(t *testing.T) {
	f := setup(t, nil)
	p := filepath.Join(f.root, ".agents/skills/test-skill/SOP.md")
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSkill(f.root, "test-skill"); err != ErrSkill {
		t.Fatal("missing SOP loaded")
	}
	target := filepath.Join(t.TempDir(), "outside.md")
	_ = os.WriteFile(target, []byte("outside"), 0600)
	_ = os.Symlink(target, p)
	if _, err := LoadSkill(f.root, "test-skill"); err != ErrSkill {
		t.Fatal("symlink SOP loaded")
	}
}

func TestRootPolicyChangeInvalidatesPinnedContract(t *testing.T) {
	f := setup(t, nil)
	if err := os.WriteFile(filepath.Join(f.root, "AGENTS.md"), []byte("Different root policy"), 0600); err != nil {
		t.Fatal(err)
	}
	if f.runtime.Dispatch(context.Background(), f.request("edit-1", "edit")) != ErrSkill || f.effects.Load() != 0 {
		t.Fatal("changed root policy executed under old contract")
	}
}
func TestVerifierPanicCannotCreateProof(t *testing.T) {
	f := setup(t, func(c *Config) { c.Checks["tests"] = func(context.Context) error { panic("SECRET_CANARY") } })
	_ = f.runtime.Submit(f.spec.TaskID, f.spec.SessionID)
	if token, err := f.runtime.Verify(context.Background(), f.spec.TaskID, f.spec.SessionID); err != ErrCheck || token != "" {
		t.Fatal("panic issued evidence")
	}
}
func TestSpecAndRegistryCopiesCannotWidenRuntimeScope(t *testing.T) {
	f := setup(t, nil)
	f.spec.Scope[0] = "unrelated"
	copy := f.runtime.Contract()
	copy.Scope[0] = "unrelated"
	copy.AllowedOperations[0] = "outside"
	if f.runtime.Dispatch(context.Background(), f.request("outside-1", "outside")) != ErrScope {
		t.Fatal("external spec alias widened scope")
	}
	snap := f.runtime.Snapshot()
	if len(snap.Audit) > 0 {
		snap.Audit[0].Decision = "forged"
	}
	if len(f.runtime.Snapshot().Audit) > 0 && f.runtime.Snapshot().Audit[0].Decision == "forged" {
		t.Fatal("snapshot mutated authority")
	}
}

func TestRegistryResourcesAreCopied(t *testing.T) {
	var external []string
	f := setup(t, func(c *Config) { external = c.Operations["outside"].Resources })
	external[0] = "internal/changed"
	if f.runtime.Dispatch(context.Background(), f.request("outside-1", "outside")) != ErrScope {
		t.Fatal("external registry mutation widened scope")
	}
}
func TestAuditRecordsTrustedVerificationAndAcceptance(t *testing.T) {
	f := setup(t, nil)
	_ = f.runtime.Dispatch(context.Background(), f.request("edit-1", "edit"))
	token := f.verify(t)
	_ = f.operator.Review(token)
	_ = f.accept(token)
	decisions := map[string]int{}
	for _, entry := range f.runtime.Snapshot().Audit {
		decisions[entry.Decision]++
	}
	if decisions["check_passed"] != 2 || decisions["operator_reviewed"] != 1 || decisions["accepted"] != 1 {
		t.Fatal("missing decision provenance")
	}
}

func TestSensitiveResourceAliasesCannotAvoidApproval(t *testing.T) {
	for _, test := range []struct {
		resource   string
		permission Kind
	}{
		{"internal/MIGRATIONS/change.sql", Migration},
		{"internal/Package.JSON", Dependency},
		{"internal/node_modules/pkg/index.js", Dependency},
		{"internal/vendor/pkg/file.go", Dependency},
		{".AGENTS", PolicyEdit},
		{"Docs/Agents/rules.md", PolicyEdit},
	} {
		t.Run(test.resource, func(t *testing.T) {
			found := false
			for _, p := range requirements(Operation{Kind: Write, Resources: []string{test.resource}}) {
				if p == test.permission {
					found = true
				}
			}
			if !found {
				t.Fatal("sensitive resource alias bypassed approval")
			}
		})
	}
	if validResource(".GIT/config") {
		t.Fatal("case alias accessed git metadata")
	}
}
func TestSkillLoaderRejectsOversizedAndBorrowedPackage(t *testing.T) {
	f := setup(t, nil)
	p := filepath.Join(f.root, ".agents/skills/test-skill/SOP.md")
	if err := os.WriteFile(p, []byte(strings.Repeat("x", 256*1024+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSkill(f.root, "test-skill"); err != ErrSkill {
		t.Fatal("oversized SOP loaded")
	}
	original := filepath.Join(f.root, ".agents/skills/test-skill")
	borrowed := filepath.Join(f.root, "borrowed")
	if err := os.Rename(original, borrowed); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(borrowed, original); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSkill(f.root, "test-skill"); err != ErrSkill {
		t.Fatal("borrowed canonical package loaded")
	}
}

func TestSuccessfulOnceOperationCannotRepeatUnderNewRequestID(t *testing.T) {
	f := setup(t, func(c *Config) { op := c.Operations["edit"]; op.OncePerRevision = true; c.Operations["edit"] = op })
	if err := f.runtime.Dispatch(context.Background(), f.request("edit-1", "edit")); err != nil {
		t.Fatal(err)
	}
	if f.runtime.Dispatch(context.Background(), f.request("edit-2", "edit")) != ErrDuplicate || f.effects.Load() != 1 {
		t.Fatal("completed work repeated")
	}
	_ = f.operator.Rebase(f.spec.Revision, f.spec.Skill)
	if f.runtime.Dispatch(context.Background(), f.request("edit-3", "edit")) != ErrDuplicate {
		t.Fatal("unchanged rebase discarded completed work")
	}
}
func TestFailedEffectNeedsHostReconciliationBeforeRetry(t *testing.T) {
	f := setup(t, func(c *Config) {
		op := c.Operations["edit"]
		op.Run = func(context.Context) error { return errors.New("unknown partial effect") }
		c.Operations["edit"] = op
	})
	if f.runtime.Dispatch(context.Background(), f.request("edit-1", "edit")) != ErrExecution {
		t.Fatal("failure hidden")
	}
	if f.runtime.Dispatch(context.Background(), f.request("edit-2", "edit")) != ErrState {
		t.Fatal("blind effect retry allowed")
	}
	if f.runtime.Submit(f.spec.TaskID, f.spec.SessionID) != ErrState {
		t.Fatal("uncertain effect submitted")
	}
	if err := f.operator.Rebase(f.spec.Revision, f.spec.Skill); err != nil {
		t.Fatal(err)
	}
	if f.runtime.Snapshot().ActionsUsed != 1 {
		t.Fatal("reconciliation reset budget")
	}
}

func TestAcceptanceModeMustBeExplicit(t *testing.T) {
	f := setup(t, nil)
	cfg := f.runtime.cfg
	cfg.Spec.AcceptanceMode = ""
	if _, _, err := New(cfg); err != ErrInvalid {
		t.Fatal("unspecified review policy allowed")
	}
	auto := setup(t, func(c *Config) { c.Spec.AcceptanceMode = TrustedChecks })
	_ = auto.runtime.Dispatch(context.Background(), auto.request("edit-1", "edit"))
	token := auto.verify(t)
	if err := auto.accept(token); err != nil {
		t.Fatal("explicit trusted-check acceptance failed", err)
	}
}

func TestDuplicateSkillBindingsAreRejected(t *testing.T) {
	f := setup(t, nil)
	data, _ := json.Marshal(f.request("edit-1", "edit"))
	original := `"policy_hash":"` + f.spec.Skill.PolicyHash + `"`
	duplicate := original + `,"policy_hash":"` + f.spec.Skill.PolicyHash + `"`
	raw := strings.Replace(string(data), original, duplicate, 1)
	if _, err := DecodeRequest([]byte(raw)); err != ErrInvalid {
		t.Fatal("duplicate skill hash accepted")
	}
}

func TestDeniedRequestDataIsNotReflectedInAudit(t *testing.T) {
	f := setup(t, nil)
	request := f.request("SECRET_CANARY", "SECRET_CANARY")
	if f.runtime.Dispatch(context.Background(), request) != ErrScope {
		t.Fatal("unregistered operation accepted")
	}
	data, _ := json.Marshal(f.runtime.Snapshot())
	if strings.Contains(string(data), "SECRET_CANARY") {
		t.Fatal("untrusted identifier reflected in audit")
	}
}

func TestCurrentEvidenceCannotBeWastefullyReverified(t *testing.T) {
	f := setup(t, nil)
	_ = f.runtime.Dispatch(context.Background(), f.request("edit-1", "edit"))
	_ = f.verify(t)
	if f.runtime.Submit(f.spec.TaskID, f.spec.SessionID) != ErrCurrentEvidence || f.checkCalls.Load() != 2 {
		t.Fatal("current checks needlessly repeated")
	}
	f.now.Store(f.now.Load().(time.Time).Add(time.Minute))
	if err := f.runtime.Submit(f.spec.TaskID, f.spec.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.runtime.Verify(context.Background(), f.spec.TaskID, f.spec.SessionID); err != nil {
		t.Fatal(err)
	}
	if f.checkCalls.Load() != 4 {
		t.Fatal("expired evidence not refreshed")
	}
}
