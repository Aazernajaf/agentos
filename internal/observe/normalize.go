// Package observe accepts only a caller-selected provider session's metadata.
package observe

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Aazernajaf/agentos/internal/fleet"
)

const MaxLine = 1024 * 1024

var ErrInput = errors.New("invalid or out-of-scope provider event")

type Normalizer struct {
	Identity   fleet.Identity
	sequence   uint64
	claudeTurn uint64
}

// Decode never forwards raw provider objects, text, tool arguments, or results.
// Sequence is a local receive counter, not a claim of provider causal ordering.
func (n *Normalizer) Decode(line []byte, at time.Time) (*fleet.Event, error) {
	if len(line) > MaxLine {
		return nil, ErrInput
	}
	line = bytes.TrimSpace(line)
	if len(line) == 0 || line[0] != '{' || !n.Identity.Valid() {
		return nil, ErrInput
	}
	var ev struct {
		Method  string `json:"method"`
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
		Session string `json:"session_id"`
		IsError *bool  `json:"is_error"`
		Stream  struct {
			Type string `json:"type"`
		} `json:"event"`
		Params struct {
			Thread string `json:"threadId"`
			Turn   struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"turn"`
		} `json:"params"`
	}
	if json.Unmarshal(line, &ev) != nil {
		return nil, ErrInput
	}
	state, turn := "", ""
	switch n.Identity.Provider {
	case "codex":
		switch ev.Method {
		case "turn/started", "turn/completed":
		default:
			return nil, nil
		}
		if ev.Params.Thread != n.Identity.Session || !fleet.ValidID(ev.Params.Turn.ID) {
			return nil, ErrInput
		}
		turn = ev.Params.Turn.ID
		if ev.Method == "turn/started" {
			state = "running"
		} else {
			switch ev.Params.Turn.Status {
			case "completed", "failed", "interrupted":
				state = ev.Params.Turn.Status
			default:
				return nil, ErrInput
			}
		}
	case "claude":
		if ev.Type != "system" && ev.Type != "stream_event" && ev.Type != "result" {
			return nil, nil
		}
		if ev.Session != n.Identity.Session {
			return nil, ErrInput
		}
		switch {
		case ev.Type == "system" && ev.Subtype == "init":
			n.claudeTurn++
			state = "unknown"
		case ev.Type == "stream_event" && ev.Stream.Type == "message_start":
			state = "running"
		case ev.Type == "result":
			if ev.IsError == nil {
				return nil, ErrInput
			}
			if *ev.IsError {
				state = "failed"
			} else if ev.Subtype == "success" {
				state = "completed"
			} else {
				return nil, ErrInput
			}
		default:
			return nil, nil
		}
		if n.claudeTurn == 0 {
			n.claudeTurn = 1
		}
		turn = fmt.Sprintf("observed-turn-%d", n.claudeTurn)
	}
	n.sequence++
	return &fleet.Event{Identity: n.Identity, Sequence: n.sequence, Turn: turn, State: state, At: at}, nil
}

func (n *Normalizer) Disconnected(at time.Time) fleet.Event {
	n.sequence++
	return fleet.Event{Identity: n.Identity, Sequence: n.sequence, State: "disconnected", At: at}
}
