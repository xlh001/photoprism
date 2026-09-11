//go:build !windows && !plan9 && !js

package proc

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blockingTreeCmd returns a shell command that starts a grandchild appending to the given file
// and then blocks, so that terminating only the direct child leaves the grandchild running.
func blockingTreeCmd(heartbeat string) *exec.Cmd {
	script := "while true; do echo tick >> " + heartbeat + "; sleep 0.05; done & sleep 30"

	// #nosec G204 -- the script is a constant with a test-owned temporary path.
	return exec.Command("/bin/sh", "-c", script)
}

// heartbeatStopped reports whether the heartbeat file stopped growing, which means no process is
// still writing to it.
func heartbeatStopped(t *testing.T, heartbeat string, wait time.Duration) bool {
	t.Helper()

	size := func() int64 {
		if s, err := os.Stat(heartbeat); err == nil {
			return s.Size()
		}
		return 0
	}

	before := size()
	time.Sleep(wait)

	return size() == before
}

func TestRun(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		var out bytes.Buffer
		cmd := exec.Command("/bin/sh", "-c", "echo done")
		cmd.Stdout = &out
		assert.NoError(t, Run(cmd, time.Minute))
		assert.Equal(t, "done", strings.TrimSpace(out.String()))
	})
	t.Run("ExitStatus", func(t *testing.T) {
		cmd := exec.Command("/bin/sh", "-c", "exit 3")
		err := Run(cmd, time.Minute)
		var exitErr *exec.ExitError
		require.True(t, errors.As(err, &exitErr))
		assert.Equal(t, 3, exitErr.ExitCode())
		assert.False(t, errors.Is(err, ErrTimeout))
	})
	t.Run("WithoutDeadline", func(t *testing.T) {
		var out bytes.Buffer
		cmd := exec.Command("/bin/sh", "-c", "echo unbounded")
		cmd.Stdout = &out
		assert.NoError(t, Run(cmd, 0))
		assert.Equal(t, "unbounded", strings.TrimSpace(out.String()))
	})
	t.Run("Timeout", func(t *testing.T) {
		cmd := exec.Command("/bin/sh", "-c", "sleep 30")
		start := time.Now()
		err := Run(cmd, 200*time.Millisecond)
		assert.True(t, errors.Is(err, ErrTimeout))
		assert.Less(t, time.Since(start), 10*time.Second)
	})
	t.Run("TerminatesGrandchild", func(t *testing.T) {
		heartbeat := filepath.Join(t.TempDir(), "heartbeat")
		cmd := blockingTreeCmd(heartbeat)
		assert.True(t, errors.Is(Run(cmd, 300*time.Millisecond), ErrTimeout))
		assert.True(t, heartbeatStopped(t, heartbeat, 500*time.Millisecond),
			"grandchild still running after the deadline")
	})
	t.Run("DirectKillLeavesGrandchild", func(t *testing.T) {
		// Control for the case above: the same tree, killed the way a plain context deadline
		// would kill it, keeps writing. Without this, TerminatesGrandchild could pass because
		// the heartbeat never started rather than because the tree was terminated.
		heartbeat := filepath.Join(t.TempDir(), "heartbeat")
		cmd := blockingTreeCmd(heartbeat)
		setProcessGroup(cmd)
		require.NoError(t, cmd.Start())
		pgid := cmd.Process.Pid
		time.Sleep(300 * time.Millisecond)
		require.NoError(t, cmd.Process.Kill())
		_ = cmd.Wait()
		survived := !heartbeatStopped(t, heartbeat, 500*time.Millisecond)
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		assert.True(t, survived, "expected the grandchild to outlive a direct kill")
	})
	t.Run("OutputAvailableAfterTimeout", func(t *testing.T) {
		// Enough output that copying it is still in flight when the deadline arrives.
		//
		// Returning before Wait leaves the copy goroutine writing to the same buffer this
		// reads, which "go test -race" reports and "make test-go" does not, so the guarantee
		// this pins is only enforced by "make test-race".
		const lines = 40000

		var out bytes.Buffer
		// #nosec G204 -- the script is a constant with a test-owned line count.
		cmd := exec.Command("/bin/sh", "-c",
			"awk 'BEGIN{for(i=0;i<"+strconv.Itoa(lines)+";i++) print \"payload-line\"}'; sleep 30")
		cmd.Stdout = &out
		assert.True(t, errors.Is(Run(cmd, 300*time.Millisecond), ErrTimeout))
		assert.Equal(t, lines, strings.Count(out.String(), "payload-line"),
			"every line the command wrote must be readable once the deadline is reported")
	})
	t.Run("Nil", func(t *testing.T) {
		assert.Error(t, Run(nil, time.Minute))
	})
	t.Run("AlreadyStarted", func(t *testing.T) {
		cmd := exec.Command("/bin/sh", "-c", "sleep 1")
		require.NoError(t, cmd.Start())
		assert.Error(t, Run(cmd, time.Minute))
		_ = cmd.Wait()
	})
}

// TestRun_ExitStatusIsNeverMasked pins that a command finishing inside its budget always
// reports its own result. Run reports a deadline only for a command it actually stopped, so a
// caller can rely on the exit status to decide whether the input or the tool was at fault.
//
// The window where the process exits in the same instant the timer fires is narrowed by the
// non-blocking re-check in Run; it cannot be scheduled deterministically, so it is not pinned
// here beyond this contract.
func TestRun_ExitStatusIsNeverMasked(t *testing.T) {
	for i := range 200 {
		cmd := exec.Command("/bin/sh", "-c", "exit 7")

		err := Run(cmd, 10*time.Second)

		var exitErr *exec.ExitError
		require.Truef(t, errors.As(err, &exitErr), "run %d reported %v", i, err)
		require.Equal(t, 7, exitErr.ExitCode())
		require.False(t, errors.Is(err, ErrTimeout))
	}
}

func TestProcessGroup(t *testing.T) {
	t.Run("LeadsItsOwnGroup", func(t *testing.T) {
		cmd := exec.Command("/bin/sh", "-c", "sleep 5")
		setProcessGroup(cmd)
		require.NoError(t, cmd.Start())
		assert.Equal(t, cmd.Process.Pid, processGroup(cmd))
		killProcessGroup(processGroup(cmd))
		_ = cmd.Wait()
	})
	t.Run("InheritedGroupIsNotReported", func(t *testing.T) {
		// A command that inherited the caller's group must report none, since signaling that
		// group would signal this process rather than the command.
		cmd := exec.Command("/bin/sh", "-c", "sleep 5")
		require.NoError(t, cmd.Start())
		assert.Zero(t, processGroup(cmd))
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	t.Run("NotStarted", func(t *testing.T) {
		assert.Zero(t, processGroup(exec.Command("/bin/sh", "-c", "true")))
		assert.Zero(t, processGroup(nil))
	})
	t.Run("SignalingNoGroupIsSafe", func(t *testing.T) {
		assert.NotPanics(t, func() {
			terminateProcessGroup(0)
			killProcessGroup(0)
			terminateProcessGroup(-1)
		})
	})
}

func TestRun_TerminateBeforeKill(t *testing.T) {
	// A command that handles SIGTERM must get the chance to exit on its own, so the grace
	// period is what separates "asked to stop" from "killed outright".
	marker := filepath.Join(t.TempDir(), "trapped")
	script := "trap 'echo stopped > " + marker + "; exit 0' TERM; while true; do sleep 0.05; done"

	// #nosec G204 -- the script is a constant with a test-owned temporary path.
	cmd := exec.Command("/bin/sh", "-c", script)

	assert.True(t, errors.Is(Run(cmd, 300*time.Millisecond), ErrTimeout))
	assert.FileExists(t, marker, "the command must be asked to terminate before it is killed")
}
