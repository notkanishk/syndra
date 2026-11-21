//go:build !unix

package main

import "os/exec"

// noHooks refuses every hook: it runs by sh -c and is killed by its process
// group, which only a Unix system has.
const noHooks = "hooks need a Unix shell"

func ownGroup(*exec.Cmd) {}
