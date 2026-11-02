package adapter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Aazernajaf/agentos/internal/governance"
)

// Exercise the real process runner behind Dispatch with an offline fake CLI.
func TestProcessLaunchAndCompletionGates(t *testing.T) {
	for _, good := range []bool{true, false} {
		name := "wrong-outcome"
		reply := "wrong"
		if good {
			name = "accepted"
			reply = marker
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			launched := filepath.Join(dir, "launched")
			script := "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo 'codex-cli 0.150.1'; exit 0; fi\n" +
				"touch '" + launched + "'\n" +
				"echo '{\"type\":\"thread.started\",\"thread_id\":\"owned\"}'\n" +
				"echo '{\"type\":\"turn.started\"}'\n" +
				"echo '{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"" + reply + "\"}}'\n" +
				"echo '{\"type\":\"turn.completed\"}'\n"
			if err := os.WriteFile(filepath.Join(dir, "codex"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
			skill, err := governance.LoadSkill("../..", "agentos-implementation")
			if err != nil {
				t.Fatal(err)
			}
			job := Job{Provider: "codex"}
			rev, _ := job.Revision()
			var evidence Evidence
			runtime, operator, err := governance.New(governance.Config{
				Spec:            governance.Spec{TaskID: "task", SessionID: "session", Revision: rev, Skill: skill, Scope: []string{"providers/codex"}, AllowedOperations: []string{"start"}, RequiredChecks: []string{"outcome"}, MaxActions: 1, MaxVerifications: 1, ExecutionTimeout: time.Second * 10, EvidenceTTL: time.Minute, AcceptanceMode: governance.TrustedChecks},
				CurrentRevision: job.Revision, CurrentSkill: func() (governance.Skill, error) { return governance.LoadSkill("../..", skill.ID) },
				Operations: map[string]governance.Operation{"start": {Kind: governance.NewSession, Resources: []string{"providers/codex"}, CapabilityVerified: true, OncePerRevision: true, Run: func(ctx context.Context) error { var err error; evidence, err = job.Run(ctx); return err }}},
				Checks:     map[string]func(context.Context) error{"outcome": func(ctx context.Context) error { return evidence.Check(ctx) }},
			})
			if err != nil {
				t.Fatal(err)
			}
			request := governance.Request{ID: "request", TaskID: "task", SessionID: "session", Revision: rev, Skill: skill, Operation: "start"}
			if !errors.Is(runtime.Dispatch(context.Background(), request), governance.ErrApproval) {
				t.Fatal("unapproved launch admitted")
			}
			if _, err := os.Stat(launched); !os.IsNotExist(err) {
				t.Fatal("denied launch spawned CLI")
			}
			if err := operator.Approve(request, []governance.Kind{governance.NewSession}); err != nil {
				t.Fatal(err)
			}
			err = runtime.Dispatch(context.Background(), request)
			if !good {
				if err == nil || runtime.Snapshot().State == "accepted" {
					t.Fatal("wrong output accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(launched); err != nil {
				t.Fatal("approved process did not run")
			}
			if !errors.Is(runtime.Dispatch(context.Background(), request), governance.ErrReplay) {
				t.Fatal("replay allowed")
			}
			if !errors.Is(runtime.Accept("task", "session", "done"), governance.ErrProof) {
				t.Fatal("claim accepted")
			}
			if err := runtime.Submit("task", "session"); err != nil {
				t.Fatal(err)
			}
			proof, err := runtime.Verify(context.Background(), "task", "session")
			if err != nil {
				t.Fatal(err)
			}
			if err := runtime.Accept("task", "session", proof); err != nil {
				t.Fatal(err)
			}
		})
	}
}
