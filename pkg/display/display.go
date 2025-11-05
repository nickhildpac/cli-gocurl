package display

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/fatih/color"
)

// PrintError prints an error message in red.
func PrintError(format string, a ...interface{}) {
	color.Red(format, a...)
}

// PrintWarning prints a warning message in yellow.
func PrintWarning(format string, a ...interface{}) {
	color.Yellow(format, a...)
}

// PrintInfo prints an informational message in blue.
func PrintInfo(format string, a ...interface{}) {
	color.Blue(format, a...)
}

// PrintSuccess prints a success message in green.
func PrintSuccess(format string, a ...interface{}) {
	color.Green(format, a...)
}

// PrintOutput prints general output.
func PrintOutput(output string) {
	fmt.Println(output)
}

// ClearScreen clears the terminal screen.
func ClearScreen() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}
