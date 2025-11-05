package executor

import (
	"os/exec"
)

// ExecuteAndCapture runs the curl command and captures its combined stdout and stderr.
func ExecuteAndCapture(args []string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}