//go:build !windows

package legacyruntime

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Standalone script runtimes own a process group. The CPA executable instead
// keeps all script workers in its host-owned group so the host's final kill
// reaches every worker even if the login executable crashes.
type processTree struct {
	mu         sync.Mutex
	ownerGroup bool
	stopped    bool
	err        error
}

func prepareProcessTree(cmd *exec.Cmd) (*processTree, error) {
	tree := &processTree{ownerGroup: os.Getenv("MQ_LOGIN_OWNED_PROCESS_GROUP") == "1"}
	if !tree.ownerGroup {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	cmd.Cancel = func() error { return tree.terminate(cmd.Process) }
	cmd.WaitDelay = 3 * time.Second
	return tree, nil
}

func (t *processTree) attach(_ *os.Process) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stopped {
		return ErrClosed
	}
	return nil
}

func (t *processTree) signalTarget(process *os.Process) int {
	if t.ownerGroup {
		return process.Pid
	}
	return -process.Pid
}

func (t *processTree) terminate(process *os.Process) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stopped {
		return t.err
	}
	t.stopped = true
	if process == nil {
		return nil
	}
	target := t.signalTarget(process)
	if err := syscall.Kill(target, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		t.err = fmt.Errorf("terminate legacy runtime worker: %w", err)
		if fallbackErr := process.Kill(); fallbackErr != nil && !errors.Is(fallbackErr, os.ErrProcessDone) {
			t.err = errors.Join(t.err, fallbackErr)
		}
		// macOS can report EPERM while sandboxed descendants are already
		// tearing down after their parent exits. Only an observed ESRCH proves
		// that this exact group is gone; otherwise preserve the kill failure.
		deadline := time.Now().Add(time.Second)
		for {
			if errors.Is(syscall.Kill(target, 0), syscall.ESRCH) {
				t.err = nil
				break
			}
			if time.Now().After(deadline) {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	return t.err
}

func (t *processTree) finalError(process *os.Process) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if process == nil {
		return t.err
	}
	target := t.signalTarget(process)
	deadline := time.Now().Add(time.Second)
	for {
		if errors.Is(syscall.Kill(target, 0), syscall.ESRCH) {
			t.err = nil
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("legacy runtime worker still exists after cleanup")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
