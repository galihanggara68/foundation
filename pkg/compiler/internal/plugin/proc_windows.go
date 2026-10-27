//go:build windows

package plugin

import (
	"os/exec"
)

// configureProcessIsolation configures process cancellation on Windows systems.
func configureProcessIsolation(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}
