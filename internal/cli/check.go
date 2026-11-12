package cli

import (
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"apidiff/internal/contract"
	"apidiff/internal/httpload"
)

const usageCheck = `usage: apidiff check --base FILE|URL --rev FILE|URL [flags]

Compare a baseline contract with a revision and report the changes.

flags:
  --base FILE|URL       baseline OpenAPI document (required)
  --rev FILE|URL        revision OpenAPI document (required)
  --format FORMAT       text (default), json or markdown
  --fail-on LEVEL       exit 1 when a finding is at or above ERR, WARN or INFO
  --ignore ID[,ID...]   rule ids to omit from the report`

func runCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintln(stderr, usageCheck) }

	var (
		base   = fs.String("base", "", "baseline document")
		rev    = fs.String("rev", "", "revision document")
		format = fs.String("format", "text", "text, json or markdown")
		failOn = fs.String("fail-on", "", "ERR, WARN or INFO")
		ignore = fs.String("ignore", "", "comma-separated rule ids")
	)
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintln(stderr, usageCheck)
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "apidiff: unexpected argument %q\n\n%s\n", fs.Arg(0), usageCheck)
		return 2
	}
	if *base == "" || *rev == "" {
		fmt.Fprintln(stderr, "apidiff: --base and --rev are required")
		fmt.Fprintln(stderr, usageCheck)
		return 2
	}
	switch *format {
	case "text", "json", "markdown":
	default:
		fmt.Fprintf(stderr, "apidiff: invalid format %q: want text, json or markdown\n", *format)
		return 2
	}

	gate := ""
	if *failOn != "" {
		gate = strings.ToUpper(*failOn)
		switch gate {
		case "ERR", "WARN", "INFO":
		default:
			fmt.Fprintf(stderr, "apidiff: invalid --fail-on %q: want ERR, WARN or INFO\n", *failOn)
			return 2
		}
	}

	baseInput, err := loadInput(*base)
	if err != nil {
		return fail(stderr, err)
	}
	revInput, err := loadInput(*rev)
	if err != nil {
		return fail(stderr, err)
	}

	res, err := contract.Compare(baseInput, revInput, contract.Options{Ignore: splitIDs(*ignore)})
	if err != nil {
		return fail(stderr, err)
	}
	if err := contract.Render(stdout, res, *format); err != nil {
		return fail(stderr, err)
	}
	if gate != "" {
		fired, err := res.FiresAt(gate)
		if err != nil {
			return fail(stderr, err)
		}
		if fired {
			return 1
		}
	}
	return 0
}

// loadInput reads a document from a file, or fetches it when the name is an
// http(s) URL. Errors always name the source the caller passed.
func loadInput(name string) (contract.Input, error) {
	if isURL(name) {
		data, err := httpload.Fetch(name)
		if err != nil {
			return contract.Input{}, fmt.Errorf("%s: %w", name, err)
		}
		return contract.Input{Name: name, Data: data}, nil
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return contract.Input{}, fmt.Errorf("read %s: %w", name, err)
	}
	return contract.Input{Name: name, Data: data}, nil
}

func isURL(name string) bool {
	u, err := url.Parse(name)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func splitIDs(list string) []string {
	var ids []string
	for _, id := range strings.Split(list, ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}
