//go:build !unix

package proc

import "os/exec"

func setProcessGroup(cmd *exec.Cmd) {}
