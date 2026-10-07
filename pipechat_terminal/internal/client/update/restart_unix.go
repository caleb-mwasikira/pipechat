//go:build !windows

package update

import (
	"os"
	"syscall"
)

// Restart starts the program at exe again, in this terminal, in place of this process. It only returns on error.
func Restart(exe string, args []string) error {
	return syscall.Exec(exe, append([]string{exe}, args...), os.Environ())
}
