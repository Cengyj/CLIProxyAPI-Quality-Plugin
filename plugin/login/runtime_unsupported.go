//go:build !darwin && !linux

package login

import "os/exec"

// Other platforms remain unavailable until descendant termination is implemented.
func runtimeSupported() bool                  { return false }
func configureRuntimeProcess(*exec.Cmd) error { return runtimeError("runtime_unavailable") }
func killRuntimeProcess(*exec.Cmd, bool)      {}
