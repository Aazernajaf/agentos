// Package adapter launches only new, disposable synthetic provider checks.
// It is not a general command runner or a sandbox for arbitrary agent work.
package adapter

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode"
)

var ErrProvider = errors.New("provider_check_failed")

const marker = "AGENTOS_GATE_OK"

// Job is host-owned and deliberately offers no arbitrary prompt/command input.
type Job struct{ Provider string }

func (j Job) Revision() (string, error) {
	if j.Provider != "codex" && j.Provider != "claude" {
		return "", ErrProvider
	}
	sum := sha256.Sum256([]byte("synthetic-gate-v1:" + j.Provider + ":" + marker))
	return hex.EncodeToString(sum[:]), nil
}

// Evidence contains no raw output, credentials, prompt or provider session IDs.
type Evidence struct {
	Stage           string   `json:"stage"`
	Provider        string   `json:"provider"`
	Version         string   `json:"version"`
	Terminal        bool     `json:"terminal_success"`
	Outcome         bool     `json:"expected_outcome"`
	StreamValid     bool     `json:"stream_valid"`
	ExitOK          bool     `json:"exit_ok"`
	Diagnostic      string   `json:"diagnostic,omitempty"`
	ParserCancelled bool     `json:"parser_cancelled"`
	Protocol        []string `json:"protocol,omitempty"`
	ErrorWords      []string `json:"error_words,omitempty"`
}

func (e Evidence) Check(context.Context) error {
	if !e.Terminal || !e.Outcome || !e.StreamValid || !e.ExitOK {
		return ErrProvider
	}
	return nil
}

func environment() []string {
	var env []string
	for _, k := range []string{"PATH", "HOME", "USER", "LOGNAME", "SHELL", "TMPDIR", "LANG", "LC_ALL", "SSL_CERT_FILE", "SSL_CERT_DIR", "HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// Run starts no process until called by the host's approved Dispatch callback.
// The outer context must also be bounded; this function caps it at 90 seconds.
func (j Job) Run(ctx context.Context) (Evidence, error) {
	e := Evidence{Provider: j.Provider, Stage: "setup"}
	if _, err := j.Revision(); err != nil {
		return e, err
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	root, err := os.MkdirTemp("", "agentos-gate-")
	if err != nil {
		return e, ErrProvider
	}
	defer os.RemoveAll(root)
	workspace := filepath.Join(root, "workspace")
	if os.Mkdir(workspace, 0700) != nil {
		return e, ErrProvider
	}
	binary, err := exec.LookPath(j.Provider)
	if err != nil {
		return e, ErrProvider
	}
	env := environment()
	e.Stage = "version"
	version, err := capture(ctx, binary, []string{"--version"}, workspace, env, 4096)
	if err != nil {
		return e, ErrProvider
	}
	expected := map[string]string{"codex": "codex-cli 0.150.1", "claude": "2.1.316 (Claude Code)"}[j.Provider]
	if strings.TrimSpace(string(version)) != expected {
		return e, ErrProvider
	}
	e.Version = expected
	var args []string
	session := ""
	if j.Provider == "codex" {
		e.Stage = "auth_setup"
		home := filepath.Join(root, "codex")
		if os.Mkdir(home, 0700) != nil {
			return e, ErrProvider
		}
		native, err := os.UserHomeDir()
		if err != nil {
			return e, ErrProvider
		}
		// Only the official CLI reads native credentials. Never copy/read their contents.
		if os.Symlink(filepath.Join(native, ".codex", "auth.json"), filepath.Join(home, "auth.json")) != nil {
			return e, ErrProvider
		}
		env = append(env, "CODEX_HOME="+home)
		args = []string{"exec", "--json", "--ephemeral", "--ignore-user-config", "--skip-git-repo-check", "--sandbox", "read-only", "-c", "approval_policy=\"never\"", "-c", "web_search=\"disabled\"", "-c", "agents.enabled=false", "-c", "allow_browser_and_computer_use=false", "-c", "model_reasoning_effort=\"low\""}
		for _, f := range []string{"shell_tool", "unified_exec", "apps", "browser_use", "browser_use_external", "computer_use", "image_generation", "multi_agent", "plugins", "hooks", "in_app_browser", "code_mode_host", "view_image", "goals", "skill_search"} {
			args = append(args, "--disable", f)
		}
	} else {
		e.Stage = "auth_status"
		auth, err := capture(ctx, binary, []string{"auth", "status", "--json"}, workspace, env, 16384)
		var status struct {
			LoggedIn bool `json:"loggedIn"`
		}
		if err != nil || json.Unmarshal(auth, &status) != nil || !status.LoggedIn {
			return e, ErrProvider
		}
		var b [16]byte
		if _, err := rand.Read(b[:]); err != nil {
			return e, ErrProvider
		}
		b[6] = (b[6] & 15) | 64
		b[8] = (b[8] & 63) | 128
		h := hex.EncodeToString(b[:])
		session = h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
		args = []string{"-p", "--output-format", "stream-json", "--verbose", "--safe-mode", "--restricted", "--tools", "", "--strict-mcp-config", "--mcp-config", "{\"mcpServers\":{}}", "--setting-sources", "", "--no-session-persistence", "--max-budget-usd", "0.25", "--effort", "low", "--session-id", session}
	}
	args = append(args, "Synthetic protocol check. Do not use tools. Reply with exactly "+marker+" and nothing else.")
	e.Stage = "provider_stream"
	cmd := command(ctx, binary, args, workspace, env)
	stderr := &bounded{limit: 65536}
	cmd.Stderr = stderr
	stream := &bounded{limit: 4*1024*1024 + 1}
	cmd.Stdout = stream
	waitErr := cmd.Run()
	terminal, outcome, parseErr := parse(strings.NewReader(string(stream.data)), j.Provider, session)
	e.StreamValid = parseErr == nil
	e.ExitOK = waitErr == nil
	e.Terminal = terminal
	e.Outcome = outcome
	e.Diagnostic = diagnose(stderr.data, stream.data)
	e.Protocol, e.ErrorWords = details(stderr.data, stream.data, session)
	if parseErr != nil || waitErr != nil || ctx.Err() != nil {
		return e, ErrProvider
	}
	// This check has no artifact output: any workspace mutation invalidates it.
	entries, err := os.ReadDir(workspace)
	if err != nil || len(entries) != 0 {
		return e, ErrProvider
	}
	if e.Check(ctx) != nil {
		return e, ErrProvider
	}
	e.Stage = "verified_outcome"
	return e, nil
}

// Details contains only literal vocabulary from this host-owned allowlist.
// Unknown types, IDs, URLs, model names and arbitrary text are never exported.
func details(stderr, stream []byte, session string) ([]string, []string) {
	known := strings.Fields("thread.started turn.started item.started item.updated item.completed turn.completed turn.failed error system init result success assistant user rate_limit_event agent_message reasoning command_execution mcp_tool_call web_search file_change text tool_use tool_result")
	allowed := map[string]bool{}
	for _, s := range known {
		allowed[s] = true
	}
	var protocol []string
	scanner := bufio.NewScanner(strings.NewReader(string(stream)))
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	errorText := string(stderr)
	for scanner.Scan() {
		var e struct {
			Type, Subtype string
			Session       string `json:"session_id"`
			IsError       bool   `json:"is_error"`
			Item          struct{ Type string }
			Result        string
			Message       json.RawMessage
			Error         json.RawMessage
			Errors        json.RawMessage
		}
		if json.Unmarshal(scanner.Bytes(), &e) != nil {
			continue
		}
		label := e.Type
		if !allowed[label] {
			label = "unknown"
		}
		if allowed[e.Subtype] {
			label += "/" + e.Subtype
		}
		if e.Item.Type != "" {
			if allowed[e.Item.Type] {
				label += "/" + e.Item.Type
			} else {
				label += "/unknown_item"
			}
		}
		if e.Session != "" {
			if e.Session == session {
				label += "/owned"
			} else {
				label += "/foreign"
			}
		}
		if e.IsError {
			label += "/error"
		}
		if len(protocol) < 24 {
			protocol = append(protocol, label)
		}
		if e.IsError || e.Type == "error" || e.Type == "turn.failed" {
			errorText += " " + e.Result + " " + string(e.Message) + " " + string(e.Error) + " " + string(e.Errors)
		}
	}
	vocabulary := strings.Fields("api error invalid request authentication unauthorized permission denied login logged in expired token missing required model effort low unsupported not supported cannot use tools tool disabled config configuration failed failure connection network timeout socket transport server response status 400 401 403 404 429 500 rate limit exceeded quota budget credit balance account organization subscription billing sandbox read only unavailable unknown parameter value no longer valid option safe mode restricted session id persistent persistence output format json stream message content thinking reasoning max tokens minimum maximum temperature tool_choice none auto is allowed must be provided enable enabled disable disabled internal unexpected argument approval policy exec")
	words := map[string]bool{}
	for _, w := range vocabulary {
		words[w] = true
	}
	var result []string
	for _, w := range strings.FieldsFunc(strings.ToLower(errorText), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if words[w] && len(result) < 120 {
			result = append(result, w)
		}
	}
	return protocol, result
}

// Export only fixed diagnostic labels, never provider text or field values.
func diagnose(stderr, stream []byte) string {
	s := strings.ToLower(string(stderr) + string(stream))
	for _, rule := range []struct{ needle, label string }{
		{"unexpected argument", "unsupported_argument"},
		{"unrecognized", "unrecognized_option"},
		{"unknown variant", "invalid_configuration_value"},
		{"error loading configuration", "configuration_error"},
		{"invalid api key", "authentication_error"},
		{"not logged in", "authentication_error"},
		{"no longer supported", "removed_configuration"},
		{"rate limit", "rate_limit"},
		{"hit your limit", "provider_usage_limit"},
		{"usage limit", "provider_usage_limit"},
		{"limit reached", "provider_usage_limit"},
		{"credit balance", "credit_balance"},
		{"max_budget", "budget_exhausted"},
		{"safe mode", "safe_mode_error"},
		{"--session-id", "session_argument_error"},
		{"--tools", "tools_argument_error"},
		{"--safe-mode", "safe_mode_argument_error"},
	} {
		if strings.Contains(s, rule.needle) {
			return rule.label
		}
	}
	if len(stream) == 0 {
		if len(stderr) > 0 {
			return "stderr_without_events"
		}
		return "no_events"
	}
	scanner := bufio.NewScanner(strings.NewReader(string(stream)))
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		var e struct {
			Type    string `json:"type"`
			Subtype string `json:"subtype"`
			IsError bool   `json:"is_error"`
		}
		if json.Unmarshal(scanner.Bytes(), &e) != nil {
			return "non_json_event"
		}
		switch e.Type {
		case "result":
			if e.IsError {
				return "error_result"
			}
			if e.Subtype != "success" {
				return "unsuccessful_result"
			}
		case "error", "turn.failed":
			return "provider_error_event"
		case "system":
			if e.Subtype != "init" {
				return "system_non_init_event"
			}
		case "thread.started", "turn.started", "item.started", "item.completed", "item.updated", "turn.completed", "assistant", "user", "rate_limit_event":
		default:
			return "unrecognized_event_type"
		}
	}
	return "recognized_event_types"
}

func command(ctx context.Context, binary string, args []string, dir string, env []string) *exec.Cmd {
	c := exec.CommandContext(ctx, binary, args...)
	c.Dir = dir
	c.Env = env
	c.Stderr = io.Discard
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error {
		if c.Process == nil {
			return nil
		}
		return syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
	}
	c.WaitDelay = time.Second
	return c
}

type bounded struct {
	data  []byte
	limit int
}

func (b *bounded) Write(p []byte) (int, error) {
	if len(b.data)+len(p) > b.limit {
		return 0, ErrProvider
	}
	b.data = append(b.data, p...)
	return len(p), nil
}
func capture(ctx context.Context, binary string, args []string, dir string, env []string, limit int) ([]byte, error) {
	b := &bounded{limit: limit}
	c := command(ctx, binary, args, dir, env)
	c.Stdout = b
	err := c.Run()
	return b.data, err
}

func parse(r io.Reader, provider, session string) (bool, bool, error) {
	scanner := bufio.NewScanner(io.LimitReader(r, 4*1024*1024+1))
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	terminal, outcome := false, false
	total := 0
	started := false
	turnStarted, messageSeen := false, false
	for scanner.Scan() {
		total += len(scanner.Bytes()) + 1
		if total > 4*1024*1024 {
			return false, false, ErrProvider
		}
		var event struct {
			Type     string `json:"type"`
			ThreadID string `json:"thread_id"`
			Session  string `json:"session_id"`
			Subtype  string `json:"subtype"`
			IsError  *bool  `json:"is_error"`
			Result   string `json:"result"`
			Item     struct {
				Type    string `json:"type"`
				Text    string `json:"text"`
				Message string `json:"message"`
			} `json:"item"`
			Message struct {
				Content []struct {
					Type string `json:"type"`
				} `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(scanner.Bytes(), &event) != nil || terminal {
			return false, false, ErrProvider
		}
		if provider == "codex" {
			switch event.Type {
			case "thread.started":
				if started || event.ThreadID == "" {
					return false, false, ErrProvider
				}
				started = true
			case "turn.started":
				if !started || turnStarted {
					return false, false, ErrProvider
				}
				turnStarted = true
			case "item.started", "item.updated", "item.completed":
				// Pinned 0.150.1 exec source maps warnings/deprecations to
				// completed error ITEMS; fatal errors are top-level events.
				// Never accept an event-gap warning or treat a warning as proof.
				if event.Type == "item.completed" && event.Item.Type == "error" && started && event.Item.Message != "" {
					message := strings.ToLower(event.Item.Message)
					if strings.Contains(message, "dropped") || strings.Contains(message, "lagged") {
						return false, false, ErrProvider
					}
					continue
				}
				if !turnStarted || (event.Item.Type != "agent_message" && event.Item.Type != "reasoning") {
					return false, false, ErrProvider
				}
				if event.Type == "item.completed" && event.Item.Type == "agent_message" {
					if messageSeen {
						return false, false, ErrProvider
					}
					messageSeen = true
					outcome = strings.TrimSpace(event.Item.Text) == marker
				}
			case "turn.completed":
				if !turnStarted {
					return false, false, ErrProvider
				}
				terminal = true
			default:
				return false, false, ErrProvider
			}
		} else {
			if event.Session != "" && event.Session != session {
				return false, false, ErrProvider
			}
			for _, c := range event.Message.Content {
				if c.Type == "tool_use" || c.Type == "tool_result" {
					return false, false, ErrProvider
				}
			}
			switch event.Type {
			case "system":
				if event.Subtype == "init" {
					if started || event.Session != session {
						return false, false, ErrProvider
					}
					started = true
				}
			case "assistant", "user", "rate_limit_event":
				if !started {
					return false, false, ErrProvider
				}
			case "result":
				if !started || event.Session != session || event.IsError == nil || *event.IsError || event.Subtype != "success" {
					return false, false, ErrProvider
				}
				terminal = true
				outcome = strings.TrimSpace(event.Result) == marker
			default:
				return false, false, ErrProvider
			}
		}
	}
	if scanner.Err() != nil || !terminal || !outcome {
		return false, false, ErrProvider
	}
	return terminal, outcome, nil
}
