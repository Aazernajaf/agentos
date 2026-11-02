// Package fleet projects scoped metadata into a bounded, in-memory fleet.
package fleet

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

const MaxSessions = 64
const MaxSubscribers = 64

var ErrInvalid = errors.New("invalid observation")
var ErrCapacity = errors.New("fleet capacity reached")

type Identity struct {
	Provider     string `json:"provider"`
	Version      string `json:"provider_version"`
	Installation string `json:"installation_id"`
	Workspace    string `json:"workspace_id"`
	Session      string `json:"session_id"`
}

type Event struct {
	Identity
	Sequence uint64
	Turn     string
	State    string
	At       time.Time
}

type Session struct {
	Identity
	Turn      string    `json:"turn_id"`
	State     string    `json:"execution_state"`
	LastKnown string    `json:"last_known_execution_state"`
	Connected bool      `json:"source_connected"`
	LastSeen  time.Time `json:"last_seen"`
	// Execution completion deliberately has no task acceptance field.
}

type Snapshot struct {
	Cursor   uint64    `json:"cursor"`
	Mode     string    `json:"mode"`
	Sessions []Session `json:"sessions"`
}

type entry struct {
	session  Session
	sequence uint64
	retired  []string
}

type Store struct {
	mu          sync.Mutex
	mode        string
	cursor      uint64
	sessions    map[string]*entry
	subscribers map[chan struct{}]struct{}
}

func New(mode string) *Store {
	return &Store{mode: mode, sessions: make(map[string]*entry), subscribers: make(map[chan struct{}]struct{})}
}

func ValidID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-_.:", c)) {
			return false
		}
	}
	return true
}

func (id Identity) Valid() bool {
	return (id.Provider == "codex" || id.Provider == "claude") && ValidID(id.Version) && ValidID(id.Installation) && ValidID(id.Workspace) && ValidID(id.Session)
}

func key(id Identity) string {
	return strings.Join([]string{id.Provider, id.Installation, id.Workspace, id.Session}, "/")
}

func (s *Store) Apply(ev Event) (bool, error) {
	if !ev.Identity.Valid() || ev.Sequence == 0 || ev.At.IsZero() {
		return false, ErrInvalid
	}
	switch ev.State {
	case "unknown", "running", "completed", "failed", "interrupted", "disconnected":
	default:
		return false, ErrInvalid
	}
	if ev.State != "unknown" && ev.State != "disconnected" && !ValidID(ev.Turn) {
		return false, ErrInvalid
	}
	if ev.Turn != "" && !ValidID(ev.Turn) {
		return false, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key(ev.Identity)
	row, exists := s.sessions[k]
	if !exists {
		if len(s.sessions) >= MaxSessions {
			return false, ErrCapacity
		}
		row = &entry{session: Session{Identity: ev.Identity, State: "unknown"}}
		s.sessions[k] = row
	}
	if ev.Sequence <= row.sequence {
		return false, nil
	}
	for _, turn := range row.retired {
		if ev.Turn != "" && turn == ev.Turn {
			return false, nil
		}
	}
	terminal := ev.State == "completed" || ev.State == "failed" || ev.State == "interrupted"
	if terminal && row.session.Turn != "" && row.session.Turn != ev.Turn {
		return false, nil
	}
	if ev.State == "running" && row.session.Turn == ev.Turn && row.session.State != "running" && row.session.State != "unknown" {
		return false, nil
	}
	if ev.State == row.session.State && ev.Turn == row.session.Turn && row.session.Connected {
		row.sequence = ev.Sequence
		return false, nil
	}
	if (ev.State == "running" || ev.State == "unknown") && row.session.Turn != "" && ev.Turn != "" && ev.Turn != row.session.Turn {
		row.retired = append(row.retired, row.session.Turn)
		if len(row.retired) > 32 {
			row.retired = row.retired[len(row.retired)-32:]
		}
	}
	row.sequence = ev.Sequence
	row.session.Identity = ev.Identity
	row.session.Connected = ev.State != "disconnected"
	row.session.LastSeen = ev.At.UTC()
	if ev.State == "disconnected" {
		row.session.State = "unknown"
	} else {
		row.session.State = ev.State
		if ev.State != "unknown" {
			row.session.LastKnown = ev.State
		}
	}
	if ev.Turn != "" {
		row.session.Turn = ev.Turn
	}
	s.cursor++
	for ch := range s.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	return true, nil
}

func (s *Store) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := Snapshot{Cursor: s.cursor, Mode: s.mode, Sessions: make([]Session, 0, len(s.sessions))}
	for _, row := range s.sessions {
		snap.Sessions = append(snap.Sessions, row.session)
	}
	sort.Slice(snap.Sessions, func(i, j int) bool { return key(snap.Sessions[i].Identity) < key(snap.Sessions[j].Identity) })
	return snap
}

// Watch coalesces changes. Consumers always receive a fresh complete snapshot.
// Subscribe before reading Snapshot to avoid a subscribe/snapshot race.
func (s *Store) Watch() (<-chan struct{}, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.subscribers) >= MaxSubscribers {
		return nil, nil, ErrCapacity
	}
	ch := make(chan struct{}, 1)
	s.subscribers[ch] = struct{}{}
	cancel := func() { s.mu.Lock(); delete(s.subscribers, ch); s.mu.Unlock() }
	return ch, cancel, nil
}
