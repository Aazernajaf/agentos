package adapter

import (
	"context"
	"strings"
	"testing"
)

func TestTerminalEvidence(t *testing.T) {
	cases := []struct {
		name, provider, stream string
		ok                     bool
	}{
		{"codex", "codex", `{"type":"thread.started","thread_id":"owned"}
{"type":"turn.started"}
{"type":"item.completed","item":{"type":"agent_message","text":"AGENTOS_GATE_OK"}}
{"type":"turn.completed"}`, true},
		{"claim-only", "codex", `{"type":"item.completed","item":{"type":"agent_message","text":"AGENTOS_GATE_OK"}}`, false},
		{"tool", "codex", `{"type":"thread.started","thread_id":"owned"}
{"type":"item.started","item":{"type":"command_execution"}}`, false},
		{"claude", "claude", `{"type":"system","subtype":"init","session_id":"owned"}
{"type":"result","subtype":"success","session_id":"owned","is_error":false,"result":"AGENTOS_GATE_OK"}`, true},
		{"auth-error-success", "claude", `{"type":"system","subtype":"init","session_id":"owned"}
{"type":"result","subtype":"success","session_id":"owned","is_error":true,"result":"AGENTOS_GATE_OK"}`, false},
		{"foreign", "claude", `{"type":"system","subtype":"init","session_id":"other"}`, false},
		{"missing-error-flag", "claude", `{"type":"system","subtype":"init","session_id":"owned"}
{"type":"result","subtype":"success","session_id":"owned","result":"AGENTOS_GATE_OK"}`, false},
		{"wrong-outcome", "claude", `{"type":"system","subtype":"init","session_id":"owned"}
{"type":"result","subtype":"success","session_id":"owned","is_error":false,"result":"done"}`, false},
		{"duplicate-terminal", "codex", `{"type":"thread.started","thread_id":"owned"}
{"type":"turn.completed"}
{"type":"turn.completed"}`, false},
		{"malformed", "codex", `not json`, false},
		{"oversized", "claude", strings.Repeat("x", 1024*1024+1), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			terminal, outcome, err := parse(strings.NewReader(tc.stream), tc.provider, "owned")
			if (err == nil) != tc.ok || (tc.ok && (!terminal || !outcome)) {
				t.Fatal("unexpected evidence decision")
			}
		})
	}
}
func TestEmptyEvidenceFails(t *testing.T) {
	if (Evidence{}).Check(context.Background()) == nil {
		t.Fatal("empty evidence accepted")
	}
}
func TestUnknownProviderDoesNotRun(t *testing.T) {
	if _, err := (Job{Provider: "other"}).Run(context.Background()); err == nil {
		t.Fatal("unknown provider accepted")
	}
}

func TestDiagnosticsNeverExportProviderText(t *testing.T) {
	for _, tc := range []struct{ stderr, stream, want string }{
		{`Error: approval_policy = "untrusted" is no longer supported; remove this setting`, "", "removed_configuration"},
		{"private account or credential", "", "stderr_without_events"},
		{"", `{"type":"result","subtype":"success","is_error":true,"result":"private response"}`, "error_result"},
		{"", `{"type":"unknown-private-type","secret":"private"}`, "unrecognized_event_type"},
		{"", `{"type":"thread.started","thread_id":"private"}`, "recognized_event_types"},
	} {
		if got := diagnose([]byte(tc.stderr), []byte(tc.stream)); got != tc.want {
			t.Fatalf("diagnostic: got %q want %q", got, tc.want)
		}
	}
}

func TestCodexWarningsAreNotFatalErrorsOrProof(t *testing.T) {
	prefix := `{"type":"thread.started","thread_id":"owned"}` + "\n"
	warning := `{"type":"item.completed","item":{"type":"error","message":"Synthetic configuration warning"}}` + "\n"
	ending := `{"type":"turn.started"}
{"type":"item.completed","item":{"type":"agent_message","text":"AGENTOS_GATE_OK"}}
{"type":"turn.completed"}`
	if _, _, err := parse(strings.NewReader(prefix+warning+ending), "codex", ""); err != nil {
		t.Fatal("documented nonfatal warning rejected")
	}
	for _, stream := range []string{
		prefix + warning,
		prefix + `{"type":"error","message":"fatal"}` + "\n" + ending,
		prefix + `{"type":"item.completed","item":{"type":"error","message":"event stream lagged; dropped 1 events"}}` + "\n" + ending,
	} {
		if _, _, err := parse(strings.NewReader(stream), "codex", ""); err == nil {
			t.Fatal("warning-only, fatal error or event gap accepted")
		}
	}
}

func TestDetailsRedactsUnknownValues(t *testing.T) {
	protocol, words := details([]byte("private-secret-token"), []byte(`{"type":"system","subtype":"init","session_id":"private-session"}
{"type":"result","subtype":"success","is_error":true,"result":"private-secret-token limit reached"}`), "private-session")
	if strings.Contains(strings.Join(append(protocol, words...), " "), "private") {
		t.Fatal("raw provider value escaped")
	}
	if len(protocol) != 2 || protocol[0] != "system/init/owned" {
		t.Fatal("missing owned attribution")
	}
}
