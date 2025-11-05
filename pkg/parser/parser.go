package parser

import (
	"github.com/google/shlex"
)

// ParseCommand splits a command line string into arguments, handling quotes.
func ParseCommand(line string) ([]string, error) {
	return shlex.Split(line)
}
