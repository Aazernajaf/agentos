// agentos-policy-demo exercises real gates with synthetic in-memory callbacks only.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/Aazernajaf/agentos/internal/governance"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Policy demo failed:", err)
		os.Exit(1)
	}
}

func run() error {
	root := flag.String("repo", ".", "AgentOS repository containing canonical skills")
	flag.Parse()
	skill, err := governance.LoadSkill(*root, "agentos-implementation")
	if err != nil {
		return err
	}
	var effects atomic.Int32
	const task, session, revision = "synthetic-task", "synthetic-session", "synthetic-revision"
	config := governance.Config{
		Spec:            governance.Spec{TaskID: task, SessionID: session, Revision: revision, Skill: skill, Scope: []string{"migrations"}, AllowedOperations: []string{"synthetic-migration"}, RequiredChecks: []string{"synthetic-acceptance"}, MaxActions: 1, MaxVerifications: 1, ExecutionTimeout: time.Second, EvidenceTTL: time.Minute, AcceptanceMode: governance.OperatorReview},
		CurrentRevision: func() (string, error) { return revision, nil },
		CurrentSkill:    func() (governance.Skill, error) { return governance.LoadSkill(*root, skill.ID) },
		Operations:      map[string]governance.Operation{"synthetic-migration": {Kind: governance.Write, Resources: []string{"migrations/synthetic-only.sql"}, Run: func(context.Context) error { effects.Add(1); return nil }}},
		Checks: map[string]func(context.Context) error{"synthetic-acceptance": func(context.Context) error {
			if effects.Load() != 1 {
				return errors.New("synthetic outcome missing")
			}
			return nil
		}},
	}
	runtime, operator, err := governance.New(config)
	if err != nil {
		return err
	}
	request := governance.Request{ID: "synthetic-request", TaskID: task, SessionID: session, Revision: revision, Skill: skill, Operation: "synthetic-migration"}
	blocked := runtime.Dispatch(context.Background(), request) == governance.ErrApproval && effects.Load() == 0
	if !blocked {
		return errors.New("unapproved callback was not blocked")
	}
	if err := operator.Approve(request, []governance.Kind{governance.Migration}); err != nil {
		return err
	}
	if err := runtime.Dispatch(context.Background(), request); err != nil {
		return err
	}
	replayBlocked := runtime.Dispatch(context.Background(), request) == governance.ErrReplay
	claimBlocked := runtime.Accept(task, session, "agent-claimed-success") == governance.ErrProof
	if !replayBlocked || !claimBlocked {
		return errors.New("replay or unverified completion accepted")
	}
	if err := runtime.Submit(task, session); err != nil {
		return err
	}
	proof, err := runtime.Verify(context.Background(), task, session)
	if err != nil {
		return err
	}
	reviewRequired := runtime.Accept(task, session, proof) == governance.ErrReview
	if !reviewRequired {
		return errors.New("operator review bypassed")
	}
	if err := operator.Review(proof); err != nil {
		return err
	}
	if err := runtime.Accept(task, session, proof); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Mode                string `json:"mode"`
		UnauthorizedBlocked bool   `json:"unapproved_action_blocked"`
		Effects             int32  `json:"approved_callback_runs"`
		ReplayBlocked       bool   `json:"replay_blocked"`
		ClaimBlocked        bool   `json:"completion_claim_blocked"`
		ReviewRequired      bool   `json:"operator_review_required"`
		FinalState          string `json:"final_state"`
	}{"synthetic-in-memory-only", blocked, effects.Load(), replayBlocked, claimBlocked, reviewRequired, runtime.Snapshot().State})
}
