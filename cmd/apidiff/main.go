// Command apidiff detects breaking changes between two OpenAPI contracts.
package main

import (
	"os"

	"apidiff/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
