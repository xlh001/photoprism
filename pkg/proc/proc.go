/*
Package proc runs external commands with a deadline and terminates the whole process tree they
create, so that a tool which delegates work to a helper cannot outlive the call that started it.
*/
package proc

import (
	"errors"
	"os/exec"
	"time"
)

// ErrTimeout is returned when a command did not finish within its deadline.
var ErrTimeout = errors.New("command timed out")

// KillGrace is how long a terminated process tree is given to exit on its own before it is
// killed outright, and how long the command is then given to release the pipes it writes to.
var KillGrace = 5 * time.Second

// Run starts the given command and waits for it to finish, terminating it and everything it
// spawned when the timeout expires. A timeout below or equal to zero runs the command without
// a deadline. The command must not have been started yet.
func Run(cmd *exec.Cmd, timeout time.Duration) error {
	if cmd == nil {
		return errors.New("command is nil")
	} else if cmd.Process != nil {
		return errors.New("command was already started")
	}

	if timeout <= 0 {
		return cmd.Run()
	}

	// Put the command in its own process group, so that the helpers it spawns can be signaled
	// together with it rather than being reparented and left running.
	setProcessGroup(cmd)

	// Output is collected through pipes, and Wait does not return while anything still holds
	// the write end. A descendant that left the process group would otherwise keep the caller
	// waiting for as long as it runs, which is the case the deadline exists to prevent.
	if cmd.WaitDelay == 0 {
		cmd.WaitDelay = KillGrace
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Resolved once, while the process is certainly alive. After Wait reaps it the id may be
	// reused, and signaling a group that is no longer this command's is worse than not signaling.
	group := processGroup(cmd)

	done := make(chan error, 1)

	go func() { done <- cmd.Wait() }()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case err := <-done:
		return err
	case <-timer.C:
	}

	// A command that finished as the timer fired reports what it did, rather than a deadline
	// it did not actually miss.
	select {
	case err := <-done:
		return err
	default:
	}

	// Ask the process group to quit, then wait for the command to report. Wait is what releases
	// the pipes and reaps the children, so it has to run whether or not the signal was accepted.
	terminateProcessGroup(group)

	grace := time.NewTimer(KillGrace)
	defer grace.Stop()

	select {
	case <-done:
		return ErrTimeout
	case <-grace.C:
	}

	killProcessGroup(group)

	<-done

	return ErrTimeout
}
