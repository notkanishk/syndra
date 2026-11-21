//go:build unix

package main

import (
	"os/exec"
	"syscall"
)

const noHooks = ""

// ownGroup runs the hook in its own process group, killed whole: a child the
// shell started never outlives the hook, and a terminal's Ctrl-C reaches the
// kit, which still puts back what it stopped, not the hook.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}
