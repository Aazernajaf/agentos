package projecttask

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) (string, Manifest) {
	t.Helper()
	dir := t.TempDir()
	for p, content := range map[string]string{"source.txt": "input", "result.txt": "real-result", "check.sh": "#!/bin/sh\ntest \"$(cat result.txt)\" = real-result\n"} {
		if err := os.WriteFile(filepath.Join(dir, p), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir, Manifest{TaskID: "project", SessionID: "owned", Inputs: []string{"source.txt", "check.sh"}, Artifacts: []Artifact{{"result.txt", digest([]byte("real-result"))}}, Tests: []Test{{"outcome-test", []string{"/bin/sh", "check.sh"}}}}
}

func TestCancelledTestNeverIssuesProof(t *testing.T) {
	dir, m := fixture(t)
	if err := os.WriteFile(filepath.Join(dir, "check.sh"), []byte("sleep 10\n"), 0600); err != nil {
		t.Fatal(err)
	}
	task, reviewer, err := New(dir, "../..", m)
	if err != nil {
		t.Fatal(err)
	}
	defer task.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	report, err := task.Verify(ctx)
	if err == nil || report.ReviewID != "" {
		t.Fatal("timed out check produced proof")
	}
	if err := reviewer.Approve(report.ReviewID); err == nil {
		t.Fatal("cancelled check accepted")
	}
}

func TestManifestRejectsAmbiguousOrImportedAuthority(t *testing.T) {
	for _, data := range []string{
		`{"task_id":"a","task_id":"b"}`,
		`{"Task_ID":"a"}`,
		`{"approved":true}`,
		`{"tests":[{"id":"one","argv":["true"],"id":"two"}]}`,
		`{} {}`,
		strings.Repeat("x", 65537),
	} {
		if _, err := DecodeManifest(strings.NewReader(data)); err == nil {
			t.Fatal("invalid manifest accepted")
		}
	}
}
func TestRealFilesTestsAndHumanReview(t *testing.T) {
	dir, m := fixture(t)
	task, reviewer, err := New(dir, "../..", m)
	if err != nil {
		t.Fatal(err)
	}
	defer task.Close()
	report, err := task.Verify(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.State != "awaiting_user_approval" || task.runtime.Snapshot().State == "accepted" || len(report.Tests) != 1 || !report.Tests[0].Passed {
		t.Fatal("automatic acceptance or missing real process evidence")
	}
	if err := task.runtime.Accept(m.TaskID, m.SessionID, task.proof); err == nil {
		t.Fatal("review bypassed")
	}
	if err := reviewer.Approve("wrong-review"); err == nil {
		t.Fatal("foreign review accepted")
	}
	if err := reviewer.Approve(report.ReviewID); err != nil {
		t.Fatal(err)
	}
	if task.Report().State != "accepted" {
		t.Fatal("explicit review did not accept")
	}
	if err := reviewer.Approve(report.ReviewID); err == nil {
		t.Fatal("approval replay accepted")
	}
}
func TestChangedEvidenceInvalidatesApproval(t *testing.T) {
	for _, p := range []string{"source.txt", "result.txt", "check.sh"} {
		t.Run(p, func(t *testing.T) {
			dir, m := fixture(t)
			task, reviewer, err := New(dir, "../..", m)
			if err != nil {
				t.Fatal(err)
			}
			defer task.Close()
			report, err := task.Verify(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, p), []byte("changed"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := reviewer.Approve(report.ReviewID); err == nil {
				t.Fatal("changed file accepted")
			}
			if task.runtime.Snapshot().State == "accepted" {
				t.Fatal("accepted stale revision")
			}
		})
	}
}
func TestFailuresNeverProduceReview(t *testing.T) {
	for _, kind := range []string{"wrong-artifact", "failing-test", "missing-file", "symlink", "mutating-test"} {
		t.Run(kind, func(t *testing.T) {
			dir, m := fixture(t)
			switch kind {
			case "wrong-artifact":
				m.Artifacts[0].SHA256 = strings.Repeat("0", 64)
			case "failing-test":
				os.WriteFile(filepath.Join(dir, "check.sh"), []byte("exit 1\n"), 0600)
			case "missing-file":
				os.Remove(filepath.Join(dir, "result.txt"))
			case "symlink":
				os.Remove(filepath.Join(dir, "result.txt"))
				os.Symlink("source.txt", filepath.Join(dir, "result.txt"))
			case "mutating-test":
				os.WriteFile(filepath.Join(dir, "check.sh"), []byte("echo changed > source.txt\nexit 0\n"), 0600)
			}
			task, reviewer, err := New(dir, "../..", m)
			if err != nil {
				return
			}
			defer task.Close()
			report, err := task.Verify(context.Background())
			if err == nil || report.ReviewID != "" {
				t.Fatal("invalid evidence issued review")
			}
			if err := reviewer.Approve(report.ReviewID); err == nil {
				t.Fatal("failed evidence accepted")
			}
		})
	}
}
func TestManifestAndReportsAreCopied(t *testing.T) {
	dir, m := fixture(t)
	task, _, err := New(dir, "../..", m)
	if err != nil {
		t.Fatal(err)
	}
	defer task.Close()
	m.Tests[0].Argv[0] = "not-a-command"
	m.Artifacts[0].SHA256 = "changed"
	report, err := task.Verify(context.Background())
	if err != nil {
		t.Fatal("caller mutated trusted configuration", err)
	}
	report.Tests[0].Passed = false
	report.Artifacts[0].SHA256 = "changed"
	if !task.Report().Tests[0].Passed || task.Report().Artifacts[0].SHA256 == "changed" {
		t.Fatal("report alias escaped")
	}
}
