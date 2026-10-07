//go:build windows

package update

import (
	"os"
	"os/exec"
)

// Restart runs the program at exe in this terminal and waits for it, then ends this process with its result:
// Windows cannot replace a process, and the shell must not get the terminal back while the new one uses it.
func Restart(exe string, args []string) error {
	command := exec.Command(exe, args...)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		return err
	}
	os.Exit(0)
	return nil
}
