package supervisor

import (
	"os"
	"os/exec"
	"sync"
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
	setPdeathsig(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	s.cmd = cmd
	return nil
}

// Restart kills the running child immediately and waits for it to be fully
// reaped before starting a fresh one — no grace period. A graceful
// SIGTERM-then-wait here would risk starting the new process (which binds
// the same address) before the old one has actually released its listening
// socket, producing "address already in use" on the new child.
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
	_ = proc.Kill()
	_ = s.cmd.Wait()
	s.cmd = nil
	return nil
}
