package observe

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Aazernajaf/agentos/internal/fleet"
)

func normalizer(provider string) *Normalizer {
	return &Normalizer{Identity: fleet.Identity{Provider: provider, Version: "test", Installation: "test", Workspace: "test", Session: "owned"}}
}
func TestCodexMetadataOnlyAndForeignScope(t *testing.T) {
	n := normalizer("codex")
	ev, err := n.Decode([]byte(`{"method":"turn/started","params":{"threadId":"owned","turn":{"id":"t1"},"text":"SECRET_CANARY","tool_input":"SECRET_CANARY"}}`), time.Now())
	if err != nil || ev == nil || ev.State != "running" {
		t.Fatal("missing running state", err)
	}
	data, _ := json.Marshal(ev)
	if strings.Contains(string(data), "SECRET_CANARY") {
		t.Fatal("payload leaked")
	}
	if _, err := n.Decode([]byte(`{"method":"turn/completed","params":{"threadId":"foreign","turn":{"id":"t1","status":"completed"}}}`), time.Now()); err != ErrInput {
		t.Fatal("foreign scope accepted")
	}
	if ev, err := n.Decode([]byte(`{"method":"future/event","text":"SECRET_CANARY"}`), time.Now()); err != nil || ev != nil {
		t.Fatal("unknown method should be ignored")
	}
}
func TestClaudeErrorFlagOverridesSuccessSubtype(t *testing.T) {
	n := normalizer("claude")
	ev, err := n.Decode([]byte(`{"type":"result","subtype":"success","session_id":"owned","is_error":true,"result":"SECRET_CANARY"}`), time.Now())
	if err != nil || ev.State != "failed" {
		t.Fatal("false success", err)
	}
	if _, err := n.Decode([]byte(`{"type":"result","session_id":"owned"}`), time.Now()); err != ErrInput {
		t.Fatal("missing error semantics accepted")
	}
}
func TestClaudeResumeChangesTurnAndRejectsInvalidInput(t *testing.T) {
	n := normalizer("claude")
	var first string
	for i := range 2 {
		ev, err := n.Decode([]byte(`{"type":"system","subtype":"init","session_id":"owned"}`), time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = ev.Turn
		} else if ev.Turn == first {
			t.Fatal("resume reused turn identity")
		}
	}
	for _, raw := range []string{`null`, `[]`, `{bad}`, strings.Repeat("x", MaxLine+1), strings.Repeat(" ", MaxLine) + `{}`} {
		if _, err := n.Decode([]byte(raw), time.Now()); err != ErrInput {
			t.Fatal("invalid input accepted")
		}
	}
	ev := n.Disconnected(time.Now())
	if ev.State != "disconnected" {
		t.Fatal("EOF not disconnected")
	}
}
