// Host-operated local project review. Never expose this command as an agent tool.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Aazernajaf/agentos/internal/projecttask"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	workspace := flag.String("workspace", ".", "real project workspace")
	policy := flag.String("policy-repo", ".", "AgentOS canonical policy repository")
	manifest := flag.String("manifest", "", "trusted operator manifest outside the project workspace")
	runChecks := flag.Bool("run-checks", false, "authorize executing all host-selected test commands; tests can have effects")
	review := flag.Bool("review", false, "wait for explicit operator confirmation on the controlling terminal after verification")
	flag.Parse()
	if !*runChecks {
		return errors.New("test_execution_requires_explicit_run_checks")
	}
	root, err := filepath.EvalSymlinks(*workspace)
	if err != nil {
		return errors.New("invalid_workspace")
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}
	config, err := filepath.EvalSymlinks(*manifest)
	if err != nil {
		return errors.New("invalid_operator_manifest")
	}
	config, err = filepath.Abs(config)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, config)
	if err != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))) {
		return errors.New("operator_manifest_must_be_outside_workspace")
	}
	file, err := os.Open(config)
	if err != nil {
		return errors.New("invalid_operator_manifest")
	}
	m, err := projecttask.DecodeManifest(file)
	file.Close()
	if err != nil {
		return err
	}
	task, reviewer, err := projecttask.New(root, *policy, m)
	if err != nil {
		return err
	}
	defer task.Close()
	report, err := task.Verify(context.Background())
	if err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(task.Report())
		return err
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		return err
	}
	if !*review {
		return nil
	}
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return errors.New("operator_terminal_required_task_not_accepted")
	}
	defer tty.Close()
	fmt.Fprintln(tty, "Review the artifact hashes and test evidence above. To accept this exact result, type: approve "+report.ReviewID)
	fmt.Fprintln(tty, "Any other response leaves the task unaccepted. Approval expires after 15 minutes.")
	line, err := bufio.NewReader(tty).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "approve "+report.ReviewID {
		return errors.New("user_approval_not_given_task_not_accepted")
	}
	if err := reviewer.Approve(report.ReviewID); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(task.Report())
}
