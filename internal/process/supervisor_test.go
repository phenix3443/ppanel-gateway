package process

import (
	"testing"
	"time"
)

func TestSupervisorStartStop(t *testing.T) {
	s := New("sleep", "30")
	if s.Running() {
		t.Fatal("not started yet")
	}
	if err := s.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !s.Running() || s.PID() == 0 {
		t.Fatalf("expected running with a PID, got running=%v pid=%d", s.Running(), s.PID())
	}
	// A second Start is a no-op (does not spawn a duplicate).
	pid := s.PID()
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	if s.PID() != pid {
		t.Fatalf("second Start spawned a new process: %d != %d", s.PID(), pid)
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if s.Running() {
		t.Fatal("still running after Stop")
	}
}

func TestSupervisorRestart(t *testing.T) {
	s := New("sleep", "30")
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	first := s.PID()
	if err := s.Restart(); err != nil {
		t.Fatalf("restart: %v", err)
	}
	second := s.PID()
	if second == 0 || second == first {
		t.Fatalf("restart should yield a new PID: first=%d second=%d", first, second)
	}
	_ = s.Stop()
	time.Sleep(10 * time.Millisecond)
}
