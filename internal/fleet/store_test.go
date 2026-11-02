package fleet

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func testEvent(seq uint64, turn, state string) Event {
	return Event{Identity: Identity{Provider: "codex", Version: "0.150.1", Installation: "test", Workspace: "synthetic", Session: "session-1"}, Sequence: seq, Turn: turn, State: state, At: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)}
}
func TestRetiredAndDuplicateEventsCannotRegressState(t *testing.T) {
	s := New("observe")
	for _, ev := range []Event{testEvent(1, "t1", "running"), testEvent(2, "t2", "running"), testEvent(3, "t1", "completed"), testEvent(4, "t1", "running"), testEvent(2, "t2", "failed")} {
		if _, err := s.Apply(ev); err != nil {
			t.Fatal(err)
		}
	}
	snap := s.Snapshot()
	if snap.Cursor != 2 || snap.Sessions[0].Turn != "t2" || snap.Sessions[0].State != "running" {
		t.Fatalf("state regressed: %+v", snap)
	}
	_, _ = s.Apply(testEvent(5, "t2", "completed"))
	_, _ = s.Apply(testEvent(6, "t2", "completed"))
	_, _ = s.Apply(testEvent(7, "t2", "running"))
	if s.Snapshot().Cursor != 3 || s.Snapshot().Sessions[0].State != "completed" {
		t.Fatal("completed turn was reopened")
	}
}
func TestDisconnectIsUnknownAndScopeIsolated(t *testing.T) {
	s := New("observe")
	_, _ = s.Apply(testEvent(1, "t1", "completed"))
	ev := testEvent(2, "", "disconnected")
	_, _ = s.Apply(ev)
	row := s.Snapshot().Sessions[0]
	if row.State != "unknown" || row.Connected || row.LastKnown != "completed" {
		t.Fatal("disconnect retained completion as current state")
	}
	ev = testEvent(1, "t1", "running")
	ev.Workspace = "another"
	_, _ = s.Apply(ev)
	if len(s.Snapshot().Sessions) != 2 {
		t.Fatal("workspace identities collided")
	}
}
func TestBoundsAndValidation(t *testing.T) {
	s := New("observe")
	bad := testEvent(1, "t1", "accepted")
	if _, err := s.Apply(bad); err != ErrInvalid {
		t.Fatal("accepted task state allowed")
	}
	for i := range MaxSessions {
		ev := testEvent(1, "t1", "running")
		ev.Session = fmt.Sprint("session-", i)
		if _, err := s.Apply(ev); err != nil {
			t.Fatal(err)
		}
	}
	ev := testEvent(1, "t1", "running")
	ev.Session = "overflow"
	if _, err := s.Apply(ev); err != ErrCapacity {
		t.Fatal("session bound ignored")
	}
	cancels := []func(){}
	for range MaxSubscribers {
		_, cancel, err := s.Watch()
		if err != nil {
			t.Fatal(err)
		}
		cancels = append(cancels, cancel)
	}
	if _, _, err := s.Watch(); err != ErrCapacity {
		t.Fatal("subscriber bound ignored")
	}
	for _, cancel := range cancels {
		cancel()
	}
}
func TestSlowWatcherAndConcurrentSnapshots(t *testing.T) {
	s := New("observe")
	ch, cancel, err := s.Watch()
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := uint64(1); i < 100; i++ {
			_, _ = s.Apply(testEvent(i, fmt.Sprint("turn-", i), "running"))
		}
	})
	wg.Go(func() {
		for range 100 {
			_ = s.Snapshot()
		}
	})
	wg.Wait()
	select {
	case <-ch:
	default:
		t.Fatal("no update signal")
	}
	if s.Snapshot().Cursor != 99 {
		t.Fatal("slow subscriber lost projection changes")
	}
}

func TestReinitializationRetiresPriorTurn(t *testing.T) {
	s := New("observe")
	for _, ev := range []Event{testEvent(1, "one", "running"), testEvent(2, "two", "unknown"), testEvent(3, "one", "running")} {
		_, _ = s.Apply(ev)
	}
	snap := s.Snapshot()
	if snap.Cursor != 2 || snap.Sessions[0].Turn != "two" {
		t.Fatal("reinitialization admitted a retired turn")
	}
}
