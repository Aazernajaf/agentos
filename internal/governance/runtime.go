// Package governance gates operations registered by trusted host code.
// It does not sandbox arbitrary code or intercept external provider tools.
package governance

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

type Kind string

const (
	Read            Kind = "read"
	Write           Kind = "write"
	CheckWork       Kind = "check"
	Dependency      Kind = "dependency"
	Migration       Kind = "migration"
	ProviderControl Kind = "provider_control"
	Push            Kind = "push"
	Deploy          Kind = "deploy"
	Release         Kind = "release"
	NewSession      Kind = "new_session"
	ExternalMessage Kind = "external_message"
	PolicyEdit      Kind = "policy_edit"
)

var (
	ErrInvalid         = errors.New("invalid_contract")
	ErrOwner           = errors.New("wrong_task_or_session")
	ErrRevision        = errors.New("revision_changed")
	ErrScope           = errors.New("outside_task_scope")
	ErrApproval        = errors.New("approval_required")
	ErrCapability      = errors.New("capability_unverified")
	ErrState           = errors.New("task_not_active")
	ErrBudget          = errors.New("action_budget_exhausted")
	ErrReplay          = errors.New("request_already_used")
	ErrDuplicate       = errors.New("operation_already_completed")
	ErrCheck           = errors.New("required_check_failed")
	ErrProof           = errors.New("missing_or_stale_proof")
	ErrCurrentEvidence = errors.New("verification_already_current")
	ErrReview          = errors.New("operator_review_required")
	ErrBusy            = errors.New("work_in_flight")
	ErrExecution       = errors.New("operation_failed")
)

type AcceptanceMode string

const (
	OperatorReview AcceptanceMode = "operator_review"
	TrustedChecks  AcceptanceMode = "trusted_checks"
)

type Spec struct {
	TaskID            string
	SessionID         string
	Revision          string
	Skill             Skill
	Scope             []string
	AllowedOperations []string
	RequiredChecks    []string
	MaxActions        int
	MaxVerifications  int
	ExecutionTimeout  time.Duration
	EvidenceTTL       time.Duration
	AcceptanceMode    AcceptanceMode
}

// Operation metadata and handlers are host-owned, never taken from an agent request.
type Operation struct {
	Kind               Kind
	Resources          []string
	OncePerRevision    bool
	CapabilityVerified bool
	Run                func(context.Context) error
}

type Config struct {
	Spec            Spec
	CurrentRevision func() (string, error)
	CurrentSkill    func() (Skill, error)
	Operations      map[string]Operation
	Checks          map[string]func(context.Context) error
	Now             func() time.Time
}

type Request struct {
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`
	SessionID string `json:"session_id"`
	Revision  string `json:"revision"`
	Skill     Skill  `json:"skill"`
	Operation string `json:"operation"`
}

type Audit struct {
	RequestID string    `json:"request_id"`
	Operation string    `json:"operation"`
	Decision  string    `json:"decision"`
	At        time.Time `json:"at"`
}

type Snapshot struct {
	TaskID            string  `json:"task_id"`
	State             string  `json:"state"`
	Revision          string  `json:"revision"`
	ActionsUsed       int     `json:"actions_used"`
	ActionLimit       int     `json:"action_limit"`
	VerificationsUsed int     `json:"verifications_used"`
	ReviewRequired    bool    `json:"review_required"`
	Audit             []Audit `json:"audit"`
}

type operationVersion struct {
	ID, Revision string
	Skill        Skill
}

type proof struct {
	token      string
	generation uint64
	at         time.Time
	reviewed   bool
}

// Runtime is agent-facing. No method imports approvals or claimed check results.
type Runtime struct {
	mu                sync.Mutex
	cfg               Config
	state             string
	paused            bool
	cancelled         bool
	generation        uint64
	completed         map[operationVersion]bool
	used              map[string]bool
	grants            map[string]Request
	proof             *proof
	running           map[string]context.CancelFunc
	verifying         bool
	verifyCancel      context.CancelFunc
	verificationCount int
	audit             []Audit
}

// Operator is a host-held capability. Never expose it via agent input or provider events.
type Operator struct{ runtime *Runtime }

func New(cfg Config) (*Runtime, *Operator, error) {
	spec := cfg.Spec

	if !keyPattern.MatchString(spec.TaskID) || !keyPattern.MatchString(spec.SessionID) || !keyPattern.MatchString(spec.Revision) || !spec.Skill.valid() {
		return nil, nil, ErrInvalid
	}
	if len(spec.Scope) == 0 || len(spec.Scope) > 64 || len(spec.AllowedOperations) == 0 || len(spec.AllowedOperations) > 64 || len(spec.RequiredChecks) == 0 || len(spec.RequiredChecks) > 64 {
		return nil, nil, ErrInvalid
	}
	if spec.MaxActions < 1 || spec.MaxActions > 1024 || spec.MaxVerifications < 1 || spec.MaxVerifications > 1024 {
		return nil, nil, ErrInvalid
	}
	if spec.ExecutionTimeout <= 0 || spec.ExecutionTimeout > 15*time.Minute || spec.EvidenceTTL <= 0 || spec.EvidenceTTL > 24*time.Hour {
		return nil, nil, ErrInvalid
	}
	if cfg.CurrentRevision == nil || cfg.CurrentSkill == nil {
		return nil, nil, ErrInvalid
	}

	if spec.AcceptanceMode != OperatorReview && spec.AcceptanceMode != TrustedChecks {
		return nil, nil, ErrInvalid
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	spec.Scope = append([]string(nil), spec.Scope...)
	spec.AllowedOperations = append([]string(nil), spec.AllowedOperations...)
	spec.RequiredChecks = append([]string(nil), spec.RequiredChecks...)
	for _, scope := range spec.Scope {
		if !validResource(scope) {
			return nil, nil, ErrInvalid
		}
	}
	ops := make(map[string]Operation, len(spec.AllowedOperations))
	for _, id := range spec.AllowedOperations {
		op, ok := cfg.Operations[id]
		if !keyPattern.MatchString(id) || !ok || op.Run == nil || !validKind(op.Kind) || len(op.Resources) == 0 || len(op.Resources) > 64 {
			return nil, nil, ErrInvalid
		}
		if _, duplicate := ops[id]; duplicate {
			return nil, nil, ErrInvalid
		}
		op.Resources = append([]string(nil), op.Resources...)
		for _, resource := range op.Resources {
			if !validResource(resource) {
				return nil, nil, ErrInvalid
			}
		}
		ops[id] = op
	}
	checks := make(map[string]func(context.Context) error, len(spec.RequiredChecks))
	for _, id := range spec.RequiredChecks {
		if !keyPattern.MatchString(id) || cfg.Checks[id] == nil {
			return nil, nil, ErrInvalid
		}
		if _, exists := checks[id]; exists {
			return nil, nil, ErrInvalid
		}
		checks[id] = cfg.Checks[id]
	}
	cfg.Spec = spec
	cfg.Operations = ops
	cfg.Checks = checks
	r := &Runtime{cfg: cfg, state: "active", used: make(map[string]bool), completed: make(map[operationVersion]bool), grants: make(map[string]Request), running: make(map[string]context.CancelFunc)}
	if err := r.fresh(); err != nil {
		return nil, nil, err
	}
	return r, &Operator{runtime: r}, nil
}

func runSafely(ctx context.Context, run func(context.Context) error) (err error) {
	defer func() {
		if recover() != nil {
			err = ErrExecution
		}
	}()
	if ctx.Err() != nil {
		return ErrExecution
	}
	return run(ctx)
}

func validKind(kind Kind) bool {
	switch kind {
	case Read, Write, CheckWork, Dependency, Migration, ProviderControl, Push, Deploy, Release, NewSession, ExternalMessage, PolicyEdit:
		return true
	}
	return false
}
func validResource(s string) bool {
	if s == "" || len(s) > 512 || s == "." || path.Clean(s) != s || strings.ContainsAny(s, "\\:\x00\r\n") || strings.HasPrefix(s, "/") || s == ".." || strings.HasPrefix(s, "../") {
		return false
	}
	for _, part := range strings.Split(s, "/") {
		if strings.EqualFold(part, ".git") {
			return false
		}
	}
	return true
}
func within(resource string, scopes []string) bool {
	for _, scope := range scopes {
		if resource == scope || strings.HasPrefix(resource, scope+"/") {
			return true
		}
	}
	return false
}

// Requirements cannot be selected/downgraded by an agent. Sensitive resources
// also require their own permission even when a host operation is called Write.
func requirements(op Operation) []Kind {
	required := map[Kind]bool{}
	switch op.Kind {
	case Read, CheckWork:
	case Write:
	default:
		required[op.Kind] = true
	}
	if op.Kind != Read && op.Kind != CheckWork {
		for _, resource := range op.Resources {
			for _, part := range strings.Split(resource, "/") {
				if strings.EqualFold(part, "migrations") {
					required[Migration] = true
				}
				if strings.EqualFold(part, "node_modules") || strings.EqualFold(part, "vendor") {
					required[Dependency] = true
				}
			}
			switch strings.ToLower(path.Base(resource)) {
			case "go.mod", "go.sum", "package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml", "bun.lock", "bun.lockb", "go.work", "go.work.sum", "npm-shrinkwrap.json", ".npmrc", ".yarnrc.yml":
				required[Dependency] = true
			}
			policyResource := strings.ToLower(resource)
			if policyResource == "agents.md" || policyResource == "claude.md" || (policyResource == ".agents" || strings.HasPrefix(policyResource, ".agents/")) || (policyResource == ".claude" || strings.HasPrefix(policyResource, ".claude/")) || (policyResource == "docs/agents" || strings.HasPrefix(policyResource, "docs/agents/")) {
				required[PolicyEdit] = true
			}
		}
	}
	result := make([]Kind, 0, len(required))
	for kind := range required {
		result = append(result, kind)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func (r *Runtime) fresh() error {
	revision, err := r.cfg.CurrentRevision()
	if err != nil || revision != r.cfg.Spec.Revision {
		return ErrRevision
	}
	skill, err := r.cfg.CurrentSkill()
	if err != nil || skill != r.cfg.Spec.Skill {
		return ErrSkill
	}
	return nil
}
func (r *Runtime) available() error {
	if r.paused || r.cancelled || r.state == "accepted" {
		return ErrState
	}
	return r.fresh()
}
func (r *Runtime) validate(req Request) (Operation, error) {
	if !keyPattern.MatchString(req.ID) || !keyPattern.MatchString(req.Operation) {
		return Operation{}, ErrInvalid
	}
	if req.TaskID != r.cfg.Spec.TaskID || req.SessionID != r.cfg.Spec.SessionID {
		return Operation{}, ErrOwner
	}
	if req.Revision != r.cfg.Spec.Revision {
		return Operation{}, ErrRevision
	}
	if req.Skill != r.cfg.Spec.Skill {
		return Operation{}, ErrSkill
	}
	if err := r.available(); err != nil {
		return Operation{}, err
	}
	op, ok := r.cfg.Operations[req.Operation]
	if !ok {
		return Operation{}, ErrScope
	}
	for _, resource := range op.Resources {
		if !within(resource, r.cfg.Spec.Scope) {
			return Operation{}, ErrScope
		}
	}
	if op.Kind == ProviderControl && !op.CapabilityVerified {
		return Operation{}, ErrCapability
	}
	if r.used[req.ID] {
		return Operation{}, ErrReplay
	}
	if op.OncePerRevision && r.completed[operationVersion{req.Operation, req.Revision, req.Skill}] {
		return Operation{}, ErrDuplicate
	}
	return op, nil
}
func (r *Runtime) record(req Request, decision string) {

	id, operation := "invalid", req.Operation
	if keyPattern.MatchString(req.ID) {
		sum := sha256.Sum256([]byte(req.ID))
		id = hex.EncodeToString(sum[:])
	}
	if _, registered := r.cfg.Operations[operation]; !registered {
		switch operation {
		case "submit", "verify", "review", "accept", "pause", "cancel", "resume", "rebase":
		default:
			operation = "unregistered"
		}
	}

	r.audit = append(r.audit, Audit{RequestID: id, Operation: operation, Decision: decision, At: r.cfg.Now().UTC()})
	if len(r.audit) > 256 {
		r.audit = r.audit[len(r.audit)-256:]
	}
}

// Dispatch checks and consumes a host approval before entering a registered handler.
func (r *Runtime) Dispatch(ctx context.Context, req Request) error {
	r.mu.Lock()
	op, err := r.validate(req)
	if err == nil && r.state != "active" {
		err = ErrState
	}
	if err == nil && len(r.running) > 0 {
		err = ErrBusy
	}
	if err == nil && len(r.used) >= r.cfg.Spec.MaxActions {
		err = ErrBudget
	}
	if err == nil && ctx.Err() != nil {
		err = ErrExecution
	}
	if err == nil && len(requirements(op)) > 0 {
		if grant, ok := r.grants[req.ID]; !ok || grant != req {
			err = ErrApproval
		}
	}
	if err != nil {
		r.record(req, err.Error())
		r.mu.Unlock()
		return err
	}
	delete(r.grants, req.ID)
	r.used[req.ID] = true
	r.generation++
	r.proof = nil
	operationCtx, cancel := context.WithTimeout(ctx, r.cfg.Spec.ExecutionTimeout)
	r.running[req.ID] = cancel
	r.record(req, "authorized")
	r.mu.Unlock()
	runErr := runSafely(operationCtx, op.Run)
	cancelled := operationCtx.Err() != nil
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.running, req.ID)
	cancel()
	if runErr != nil || cancelled || r.paused || r.cancelled {
		r.state = "needs_reconciliation"
		r.record(req, "execution_failed")
		return ErrExecution
	}
	if op.OncePerRevision {
		r.completed[operationVersion{req.Operation, req.Revision, req.Skill}] = true
	}
	r.record(req, "executed")
	return nil
}

// Approve requires every protected category and binds exactly one request.
func (o *Operator) Approve(req Request, permissions []Kind) error {
	r := o.runtime
	r.mu.Lock()
	defer r.mu.Unlock()
	op, err := r.validate(req)
	if err != nil {
		return err
	}
	if r.state != "active" {
		return ErrState
	}
	if _, exists := r.grants[req.ID]; !exists && len(r.grants) >= r.cfg.Spec.MaxActions {
		return ErrBudget
	}
	supplied := map[Kind]bool{}
	for _, permission := range permissions {
		if !validKind(permission) {
			return ErrInvalid
		}
		supplied[permission] = true
	}
	for _, permission := range requirements(op) {
		if !supplied[permission] {
			return ErrApproval
		}
	}
	r.grants[req.ID] = req
	r.record(req, "operator_approved")
	return nil
}

func (r *Runtime) Submit(task, session string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if task != r.cfg.Spec.TaskID || session != r.cfg.Spec.SessionID {
		return ErrOwner
	}
	if err := r.available(); err != nil {
		return err
	}
	if r.state != "active" && r.state != "verified" {
		return ErrState
	}
	if len(r.running) > 0 || r.verifying {
		return ErrBusy
	}
	if r.state == "verified" && r.proof != nil {
		age := r.cfg.Now().Sub(r.proof.at)
		if age < 0 {
			return ErrProof
		}
		if age < r.cfg.Spec.EvidenceTTL {
			return ErrCurrentEvidence
		}
	}
	r.proof = nil
	r.generation++
	r.state = "submitted"
	r.recordTask("submit", "submitted")
	return nil
}

// Verify runs only the host's required check functions. Agent claims are not input.
func (r *Runtime) Verify(ctx context.Context, task, session string) (string, error) {
	r.mu.Lock()
	if task != r.cfg.Spec.TaskID || session != r.cfg.Spec.SessionID {
		r.mu.Unlock()
		return "", ErrOwner
	}
	if err := r.available(); err != nil {
		r.mu.Unlock()
		return "", err
	}
	if r.state != "submitted" {
		r.mu.Unlock()
		return "", ErrState
	}
	if len(r.running) > 0 || r.verifying {
		r.mu.Unlock()
		return "", ErrBusy
	}
	if r.verificationCount >= r.cfg.Spec.MaxVerifications {
		r.mu.Unlock()
		return "", ErrBudget
	}
	r.verificationCount++
	checkCtx, cancelChecks := context.WithTimeout(ctx, r.cfg.Spec.ExecutionTimeout)
	defer cancelChecks()
	r.verifyCancel = cancelChecks
	r.verifying = true
	r.state = "verifying"
	r.recordTask("verify", "verification_started")
	generation := r.generation
	checks := make([]func(context.Context) error, 0, len(r.cfg.Spec.RequiredChecks))
	for _, id := range r.cfg.Spec.RequiredChecks {
		checks = append(checks, r.cfg.Checks[id])
	}
	r.mu.Unlock()
	var checkErr error
	outcomes := make([]Audit, 0, len(checks))
	for index, check := range checks {
		if checkCtx.Err() != nil {
			checkErr = ErrCheck
			outcomes = append(outcomes, Audit{RequestID: r.cfg.Spec.RequiredChecks[index], Operation: "verify", Decision: "check_not_run"})
			break
		}
		if runSafely(checkCtx, check) != nil || checkCtx.Err() != nil {
			checkErr = ErrCheck
			outcomes = append(outcomes, Audit{RequestID: r.cfg.Spec.RequiredChecks[index], Operation: "verify", Decision: "check_failed"})
			break
		}
		outcomes = append(outcomes, Audit{RequestID: r.cfg.Spec.RequiredChecks[index], Operation: "verify", Decision: "check_passed"})
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, outcome := range outcomes {
		r.record(Request{ID: outcome.RequestID, Operation: outcome.Operation}, outcome.Decision)
	}
	r.verifying = false
	r.verifyCancel = nil
	if err := r.available(); err != nil {
		r.state = "active"
		r.recordTask("verify", err.Error())
		return "", err
	}
	if r.generation != generation || checkCtx.Err() != nil {
		r.state = "active"
		r.recordTask("verify", "verification_invalidated")
		return "", ErrProof
	}
	if checkErr != nil {
		r.state = "active"
		r.proof = nil
		r.recordTask("verify", "verification_failed")
		return "", ErrCheck
	}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		r.state = "active"
		r.recordTask("verify", "verification_invalidated")
		return "", ErrProof
	}
	r.proof = &proof{token: hex.EncodeToString(token), generation: generation, at: r.cfg.Now()}
	r.state = "verified"
	r.recordTask("verify", "verified")
	return r.proof.token, nil
}

func (r *Runtime) proofValid(token string) error {
	if err := r.available(); err != nil {
		return err
	}
	if r.state != "verified" || r.proof == nil || r.proof.generation != r.generation || subtle.ConstantTimeCompare([]byte(token), []byte(r.proof.token)) != 1 {
		return ErrProof
	}
	age := r.cfg.Now().Sub(r.proof.at)
	if age < 0 || age >= r.cfg.Spec.EvidenceTTL {
		return ErrProof
	}
	return nil
}
func (o *Operator) Review(token string) error {
	r := o.runtime
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.proofValid(token); err != nil {
		return err
	}
	r.proof.reviewed = true
	r.recordTask("review", "operator_reviewed")
	return nil
}
func (r *Runtime) Accept(task, session, token string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if task != r.cfg.Spec.TaskID || session != r.cfg.Spec.SessionID {
		return ErrOwner
	}
	if err := r.proofValid(token); err != nil {
		return err
	}
	if len(r.running) > 0 || r.verifying {
		return ErrBusy
	}
	if r.cfg.Spec.AcceptanceMode == OperatorReview && !r.proof.reviewed {
		return ErrReview
	}
	r.state = "accepted"
	r.recordTask("accept", "accepted")
	r.proof = nil
	r.grants = make(map[string]Request)
	return nil
}

func (o *Operator) Pause()  { o.stop(false) }
func (o *Operator) Cancel() { o.stop(true) }
func (o *Operator) stop(cancelled bool) {
	r := o.runtime
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == "accepted" {
		return
	}
	r.generation++
	r.proof = nil
	r.grants = make(map[string]Request)
	if cancelled {
		r.cancelled = true
		r.recordTask("cancel", "cancelled")
	} else {
		r.paused = true
		r.recordTask("pause", "paused")
	}
	if r.verifyCancel != nil {
		r.verifyCancel()
	}
	for _, cancel := range r.running {
		cancel()
	}
}
func (o *Operator) Resume() error {
	r := o.runtime
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancelled || r.state == "accepted" || !r.paused {
		return ErrState
	}
	if err := r.fresh(); err != nil {
		return err
	}
	if len(r.running) > 0 || r.verifying {
		return ErrBusy
	}
	r.paused = false
	if r.state != "needs_reconciliation" {
		r.state = "active"
	}
	r.recordTask("resume", "resumed")
	return nil
}

// Rebase acknowledges current inputs, invalidates approvals/proof, and preserves budget.
func (o *Operator) Rebase(revision string, skill Skill) error {
	r := o.runtime
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancelled || r.state == "accepted" {
		return ErrState
	}
	if len(r.running) > 0 || r.verifying {
		return ErrBusy
	}
	current, err := r.cfg.CurrentRevision()
	if err != nil || current != revision || !keyPattern.MatchString(revision) {
		return ErrRevision
	}
	loaded, err := r.cfg.CurrentSkill()
	if err != nil || loaded != skill || !skill.valid() {
		return ErrSkill
	}
	r.cfg.Spec.Revision = revision
	r.cfg.Spec.Skill = skill
	r.generation++
	r.proof = nil
	r.grants = make(map[string]Request)
	r.state = "active"
	r.recordTask("rebase", "rebased")
	return nil
}
func (r *Runtime) Snapshot() Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.state
	if r.cancelled {
		state = "cancelled"
	} else if r.paused {
		state = "paused"
	}
	return Snapshot{TaskID: r.cfg.Spec.TaskID, State: state, Revision: r.cfg.Spec.Revision, ActionsUsed: len(r.used), ActionLimit: r.cfg.Spec.MaxActions, VerificationsUsed: r.verificationCount, ReviewRequired: r.cfg.Spec.AcceptanceMode == OperatorReview, Audit: append([]Audit(nil), r.audit...)}
}

func (r *Runtime) recordTask(operation, decision string) {
	r.record(Request{ID: "task", Operation: operation}, decision)
}

// Contract returns metadata copies, never operator authority or receipt secrets.
func (r *Runtime) Contract() Spec {
	r.mu.Lock()
	defer r.mu.Unlock()
	spec := r.cfg.Spec
	spec.Scope = append([]string(nil), spec.Scope...)
	spec.AllowedOperations = append([]string(nil), spec.AllowedOperations...)
	spec.RequiredChecks = append([]string(nil), spec.RequiredChecks...)
	return spec
}
