// RECONSTRUCTION — not original source. See ../../../REVERSE-ENGINEERING.md.
//
// Mirrors the child-process supervision observed live (internal/module/module.go
// "启动模块 ... 参数: [run]", PID logging) and the stop/restart used by
// UpdateModule (0x7e67e0): the gateway runs each module binary as "<bin> run",
// tracks its PID, and stops it before an update then restarts it after.
package process

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Supervisor runs and supervises a single child process ("<bin> args...").
type Supervisor struct {
	bin  string
	args []string

	mu  sync.Mutex
	cmd *exec.Cmd
}

// New builds a supervisor for bin with args. [E] observed args are ["run"].
func New(bin string, args ...string) *Supervisor {
	return &Supervisor{bin: bin, args: args}
}

// Running reports whether the child is currently started.
func (s *Supervisor) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cmd != nil && s.cmd.Process != nil
}

// PID returns the child PID, or 0 when not running.
func (s *Supervisor) PID() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil || s.cmd.Process == nil {
		return 0
	}
	return s.cmd.Process.Pid
}

// Start launches the child if it is not already running. Its stdout/stderr are
// inherited so the module's logs surface alongside the gateway's, matching the
// observed single-container behavior.
func (s *Supervisor) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil && s.cmd.Process != nil {
		return nil
	}
	cmd := exec.Command(s.bin, s.args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", s.bin, err)
	}
	s.cmd = cmd
	go func() { _ = cmd.Wait() }() // reap on its own exit
	return nil
}

// Stop signals the child to terminate and waits up to grace for it, escalating
// to SIGKILL. Returns nil when no child is running.
func (s *Supervisor) Stop() error {
	return s.stop(5 * time.Second)
}

func (s *Supervisor) stop(grace time.Duration) error {
	s.mu.Lock()
	cmd := s.cmd
	s.cmd = nil
	s.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() { _, _ = cmd.Process.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(grace):
		_ = cmd.Process.Kill()
		<-done
	}
	return nil
}

// Restart stops the current child (if any) and starts a fresh one.
func (s *Supervisor) Restart() error {
	if err := s.Stop(); err != nil {
		return err
	}
	return s.Start()
}
