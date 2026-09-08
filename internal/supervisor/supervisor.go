package supervisor

import (
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Supervisor owns the lifecycle of exactly one child process (in
// production, ./manage.py runsslserver <addr>), restarting it on demand.
type Supervisor struct {
	dir  string
	name string
	args []string

	mu  sync.Mutex
	cmd *exec.Cmd
}

func New(dir, name string, args ...string) *Supervisor {
	return &Supervisor{dir: dir, name: name, args: args}
}

func (s *Supervisor) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startLocked()
}

func (s *Supervisor) startLocked() error {
	cmd := exec.Command(s.name, s.args...)
	cmd.Dir = s.dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	s.cmd = cmd
	return nil
}

// Restart terminates the running child (SIGTERM, then SIGKILL after a
// grace period if it doesn't exit) and starts a fresh one.
func (s *Supervisor) Restart() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.stopLocked(); err != nil {
		return err
	}
	return s.startLocked()
}

func (s *Supervisor) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopLocked()
}

func (s *Supervisor) stopLocked() error {
	if s.cmd == nil || s.cmd.Process == nil {
		return nil
	}
	proc := s.cmd.Process
	_ = proc.Signal(syscall.SIGTERM)

	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = proc.Kill()
		<-done
	}
	s.cmd = nil
	return nil
}
