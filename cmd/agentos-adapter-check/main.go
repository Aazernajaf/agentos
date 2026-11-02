// agentos-adapter-check is a host-operated synthetic check, not an agent API.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Aazernajaf/agentos/internal/adapter"
	"github.com/Aazernajaf/agentos/internal/governance"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	provider := flag.String("provider", "", "codex or claude")
	root := flag.String("repo", ".", "repository skill root")
	live := flag.Bool("live", false, "explicit consent to one synthetic model call (90 seconds; Claude $0.25 cap)")
	approve := flag.Bool("approve-new-session", false, "host approval for this one new session")
	flag.Parse()
	job := adapter.Job{Provider: *provider}
	revision, err := job.Revision()
	if err != nil {
		return err
	}
	skill, err := governance.LoadSkill(*root, "agentos-implementation")
	if err != nil {
		return err
	}
	var evidence adapter.Evidence
	calls := 0
	const task, session, op = "adapter-check", "owned-synthetic-session", "start-synthetic"
	runtime, operator, err := governance.New(governance.Config{
		Spec:            governance.Spec{TaskID: task, SessionID: session, Revision: revision, Skill: skill, Scope: []string{"providers/" + *provider}, AllowedOperations: []string{op}, RequiredChecks: []string{"provider-outcome"}, MaxActions: 1, MaxVerifications: 1, ExecutionTimeout: 90 * time.Second, EvidenceTTL: time.Minute, AcceptanceMode: governance.TrustedChecks},
		CurrentRevision: job.Revision, CurrentSkill: func() (governance.Skill, error) { return governance.LoadSkill(*root, skill.ID) },
		Operations: map[string]governance.Operation{op: {Kind: governance.NewSession, Resources: []string{"providers/" + *provider}, OncePerRevision: true, Run: func(ctx context.Context) error { calls++; var err error; evidence, err = job.Run(ctx); return err }}},
		Checks: map[string]func(context.Context) error{"provider-outcome": func(ctx context.Context) error {
			if calls != 1 {
				return adapter.ErrProvider
			}
			return evidence.Check(ctx)
		}},
	})
	if err != nil {
		return err
	}
	req := governance.Request{ID: "one-launch", TaskID: task, SessionID: session, Revision: revision, Skill: skill, Operation: op}
	blocked := errors.Is(runtime.Dispatch(context.Background(), req), governance.ErrApproval) && calls == 0
	if !blocked {
		return errors.New("permission_boundary_failed")
	}
	if !*live || !*approve {
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"provider": *provider, "unapproved_launch_blocked": blocked, "provider_calls": calls, "state": "awaiting_explicit_live_approval"})
	}
	if err := operator.Approve(req, []governance.Kind{governance.NewSession}); err != nil {
		return err
	}
	if err := runtime.Dispatch(context.Background(), req); err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(evidence)
		return err
	}
	claimBlocked := errors.Is(runtime.Accept(task, session, "provider-said-done"), governance.ErrProof)
	if !claimBlocked {
		return errors.New("completion_boundary_failed")
	}
	if err := runtime.Submit(task, session); err != nil {
		return err
	}
	proof, err := runtime.Verify(context.Background(), task, session)
	if err != nil {
		return err
	}
	if err := runtime.Accept(task, session, proof); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Evidence     adapter.Evidence `json:"evidence"`
		Blocked      bool             `json:"unapproved_launch_blocked"`
		ClaimBlocked bool             `json:"unverified_completion_blocked"`
		Calls        int              `json:"provider_calls"`
		State        string           `json:"state"`
	}{evidence, blocked, claimBlocked, calls, runtime.Snapshot().State})
}
