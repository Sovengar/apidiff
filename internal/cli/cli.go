// Package cli implements the apidiff command line: flags, exit codes and
// output. Diagnostics go to stderr; only the report goes to stdout.
package cli

import (
	"fmt"
	"io"
)

const usageRoot = `usage: apidiff <command> [flags]

commands:
  check    compare a baseline contract with a revision
  capture  fetch a live contract into a file

run "apidiff <command> --help" for the flags of a command`

// Run executes apidiff with the given arguments (without the program name).
// It returns the process exit code: 0 ok, 1 gate fired, 2 usage/IO/parse error.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usageRoot)
		return 2
	}
	switch args[0] {
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "capture":
		return runCapture(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprintln(stdout, usageRoot)
		return 0
	default:
		fmt.Fprintf(stderr, "apidiff: unknown command %q\n\n%s\n", args[0], usageRoot)
		return 2
	}
}

func fail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "apidiff: %v\n", err)
	return 2
}
