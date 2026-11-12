package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"apidiff/internal/httpload"
	"go.yaml.in/yaml/v3"
)

const usageCapture = `usage: apidiff capture --url URL --out FILE [--force]

Fetch a live OpenAPI document and write it as pretty-printed JSON.

flags:
  --url URL    document to fetch (required)
  --out FILE   destination file (required)
  --force      overwrite an existing file`

func runCapture(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("capture", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintln(stderr, usageCapture) }

	var (
		url   = fs.String("url", "", "document to fetch")
		out   = fs.String("out", "", "destination file")
		force = fs.Bool("force", false, "overwrite an existing file")
	)
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintln(stderr, usageCapture)
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "apidiff: unexpected argument %q\n\n%s\n", fs.Arg(0), usageCapture)
		return 2
	}
	if *url == "" || *out == "" {
		fmt.Fprintln(stderr, "apidiff: --url and --out are required")
		fmt.Fprintln(stderr, usageCapture)
		return 2
	}

	if !*force {
		if _, err := os.Stat(*out); err == nil {
			return fail(stderr, fmt.Errorf("output file %s already exists (use --force to overwrite)", *out))
		} else if !errors.Is(err, os.ErrNotExist) {
			return fail(stderr, fmt.Errorf("stat %s: %w", *out, err))
		}
	}

	data, err := httpload.Fetch(*url)
	if err != nil {
		return fail(stderr, fmt.Errorf("%s: %w", *url, err))
	}
	if !isOpenAPI(data) {
		return fail(stderr, fmt.Errorf("response from %s is not an OpenAPI document", *url))
	}
	pretty, err := prettyJSON(data)
	if err != nil {
		return fail(stderr, fmt.Errorf("encode %s: %w", *url, err))
	}
	if err := writeAtomic(*out, pretty); err != nil {
		return fail(stderr, fmt.Errorf("write %s: %w", *out, err))
	}
	fmt.Fprintf(stdout, "wrote %s\n", *out)
	return 0
}

// isOpenAPI sniffs by content: a top-level openapi or swagger key. JSON is a
// subset of YAML, so one parser covers both.
func isOpenAPI(data []byte) bool {
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false
	}
	if doc == nil {
		return false
	}
	if _, ok := doc["openapi"]; ok {
		return true
	}
	_, ok := doc["swagger"]
	return ok
}

func prettyJSON(data []byte) ([]byte, error) {
	if json.Valid(data) {
		var buf bytes.Buffer
		if err := json.Indent(&buf, data, "", "  "); err != nil {
			return nil, err
		}
		buf.WriteByte('\n')
		return buf.Bytes(), nil
	}
	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	encoded, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

// writeAtomic writes to a temp file in the target directory and renames it, so
// a failed write never leaves a partial document behind.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".apidiff-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
