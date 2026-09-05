// Package background implements the -B flag: re-launch the current
// download detached from the terminal, with its output redirected to a
// log file, while the original process reports the log location and
// exits immediately.
package background

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

const LogFile = "wget-log"

// Relaunch starts a detached copy of the current executable with args
// (which must not include -B, or it would recurse) and redirects its
// stdout/stderr to LogFile.
func Relaunch(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not resolve executable: %w", err)
	}

	logFile, err := os.OpenFile(LogFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("could not create log file: %w", err)
	}
	defer logFile.Close()

	cmd := exec.Command(exe, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	// Detach into its own session so the child survives the parent
	// (and its controlling terminal) exiting right after Start.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start background download: %w", err)
	}

	fmt.Printf("Output will be written to %q.\n", LogFile)
	return nil
}
