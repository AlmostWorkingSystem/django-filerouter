package supervisor

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// fakeServerScript returns an absolute path to testdata/fake_server.sh.
// Absolute matters here: Supervisor sets cmd.Dir to a scratch temp dir, and
// relying on a relative script path being resolved against the *test's*
// working directory rather than cmd.Dir would be exactly the kind of
// implicit-behavior assumption this codebase's CLAUDE.md warns against —
// resolve it explicitly instead.
func fakeServerScript(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("testdata/fake_server.sh")
	if err != nil {
		t.Fatalf("resolving fake_server.sh path: %v", err)
	}
	return abs
}

func readPID(t *testing.T, path string) int {
	t.Helper()
	var data []byte
	var err error
	for i := 0; i < 50; i++ {
		data, err = os.ReadFile(path)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("reading pid file %s: %v", path, err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("parsing pid: %v", err)
	}
	return pid
}

func TestSupervisor_StartAndRestart(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")

	s := New(dir, fakeServerScript(t), pidFile)
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop()

	firstPID := readPID(t, pidFile)

	os.Remove(pidFile)
	if err := s.Restart(); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	secondPID := readPID(t, pidFile)

	if firstPID == secondPID {
		t.Fatalf("want a new process after Restart, got same pid %d twice", firstPID)
	}
}

func TestSupervisor_StopTerminatesChild(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")

	s := New(dir, fakeServerScript(t), pidFile)
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	pid := readPID(t, pidFile)

	if err := s.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	proc, _ := os.FindProcess(pid)
	// On Unix, signaling with syscall.Signal(0) probes liveness without
	// actually sending a signal — it still fails with ESRCH if the pid is
	// gone, which is exactly what Stop should have caused.
	if err := proc.Signal(syscall.Signal(0)); err == nil {
		t.Fatalf("want child process %d to be gone after Stop", pid)
	}
}
