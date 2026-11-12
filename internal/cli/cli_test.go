package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestCheckDefaultIsReportOnly(t *testing.T) {
	code, out, errOut := run(t, "check",
		"--base", "../contract/testdata/v1.yaml", "--rev", "../contract/testdata/v2.yaml")
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, errOut)
	}
	if !strings.Contains(out, "api-path-removed-without-deprecation") {
		t.Errorf("stdout missing breaking change:\n%s", out)
	}
	if !strings.Contains(out, "endpoint-added") {
		t.Errorf("stdout missing non-breaking change:\n%s", out)
	}
}

func TestCheckFailOnERRGatesBreaking(t *testing.T) {
	code, _, errOut := run(t, "check",
		"--base", "../contract/testdata/v1.yaml", "--rev", "../contract/testdata/v2.yaml", "--fail-on", "ERR")
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%q", code, errOut)
	}
}

func TestCheckFailOnERRPassesAdditive(t *testing.T) {
	code, _, stderr := run(t, "check",
		"--base", "../contract/testdata/v1.yaml", "--rev", "../contract/testdata/additive.yaml", "--fail-on", "ERR")
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr)
	}
}

func TestCheckFailOnLevels(t *testing.T) {
	// Additive changes are INFO: WARN must not fire, INFO must.
	if code, _, _ := run(t, "check", "--base", "../contract/testdata/v1.yaml", "--rev", "../contract/testdata/additive.yaml", "--fail-on", "WARN"); code != 0 {
		t.Errorf("additive with --fail-on WARN exit = %d, want 0", code)
	}
	if code, _, _ := run(t, "check", "--base", "../contract/testdata/v1.yaml", "--rev", "../contract/testdata/additive.yaml", "--fail-on", "INFO"); code != 1 {
		t.Errorf("additive with --fail-on INFO exit = %d, want 1", code)
	}
}

func TestCheckIgnoreRuleID(t *testing.T) {
	code, out, _ := run(t, "check",
		"--base", "../contract/testdata/v1.yaml", "--rev", "../contract/testdata/additive.yaml",
		"--ignore", "endpoint-added")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if strings.Contains(out, "endpoint-added") {
		t.Errorf("ignored rule still in output:\n%s", out)
	}
}

func TestCheckMalformedRevisionIsActionable(t *testing.T) {
	code, _, errOut := run(t, "check",
		"--base", "../contract/testdata/v1.yaml", "--rev", "../contract/testdata/malformed.yaml")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "malformed.yaml") {
		t.Errorf("stderr does not name malformed.yaml: %q", errOut)
	}
	assertNoStackTrace(t, errOut)
}

func TestCheckMissingFileNamesPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.yaml")
	code, _, errOut := run(t, "check", "--base", missing, "--rev", "../contract/testdata/v1.yaml")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, missing) {
		t.Errorf("stderr does not name %s: %q", missing, errOut)
	}
	assertNoStackTrace(t, errOut)
}

func TestCheckJSONFormat(t *testing.T) {
	code, out, errOut := run(t, "check",
		"--base", "../contract/testdata/v1.yaml", "--rev", "../contract/testdata/v2.yaml", "--format", "json")
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, errOut)
	}
	if strings.ContainsRune(out, '\x1b') {
		t.Fatalf("stdout contains ANSI: %q", out)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, out)
	}
	for _, key := range []string{"base", "revision", "breaking", "nonBreaking"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("JSON missing key %q", key)
		}
	}
	if doc["base"] != "../contract/testdata/v1.yaml" || doc["revision"] != "../contract/testdata/v2.yaml" {
		t.Errorf("base/revision = %v/%v, want caller-provided names", doc["base"], doc["revision"])
	}
}

func TestCheckMarkdownFormat(t *testing.T) {
	code, out, errOut := run(t, "check",
		"--base", "../contract/testdata/v1.yaml", "--rev", "../contract/testdata/v2.yaml", "--format", "markdown")
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, errOut)
	}
	if !strings.Contains(out, "## API contract") {
		t.Errorf("markdown missing heading:\n%s", out)
	}
	if !strings.Contains(out, "/paths/~1widgets~1{id}/get") {
		t.Errorf("markdown missing breaking pointer:\n%s", out)
	}
	if strings.Contains(out, "endpoint-added") {
		t.Errorf("markdown lists non-breaking entries:\n%s", out)
	}
}

func TestCheckUnknownFormatIsUsageError(t *testing.T) {
	code, _, errOut := run(t, "check",
		"--base", "../contract/testdata/v1.yaml", "--rev", "../contract/testdata/v2.yaml", "--format", "yaml")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	assertNoStackTrace(t, errOut)
}

func TestCheckMissingBaseShowsUsage(t *testing.T) {
	code, _, errOut := run(t, "check", "--rev", "../contract/testdata/v1.yaml")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "usage: apidiff check") {
		t.Errorf("stderr does not show check usage: %q", errOut)
	}
	assertNoStackTrace(t, errOut)
}

func TestCheckMissingRevShowsUsage(t *testing.T) {
	code, _, errOut := run(t, "check", "--base", "../contract/testdata/v1.yaml")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "usage: apidiff check") {
		t.Errorf("stderr does not show check usage: %q", errOut)
	}
}

func TestCheckRevisionFromURL(t *testing.T) {
	fixture, err := os.ReadFile("../contract/testdata/v2.yaml")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	code, out, errOut := run(t, "check", "--base", "../contract/testdata/v1.yaml", "--rev", srv.URL)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, errOut)
	}
	if !strings.Contains(out, "api-path-removed-without-deprecation") {
		t.Errorf("stdout missing breaking change from URL revision:\n%s", out)
	}
}

func TestCheckURLRevisionFailureIsActionable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	code, _, errOut := run(t, "check", "--base", "../contract/testdata/v1.yaml", "--rev", srv.URL)
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, srv.URL) {
		t.Errorf("stderr does not name the URL: %q", errOut)
	}
	assertNoStackTrace(t, errOut)
}

func TestCaptureWritesPrettyJSON(t *testing.T) {
	fixture, err := os.ReadFile("../contract/testdata/v2.yaml")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	out := filepath.Join(t.TempDir(), "baseline.json")
	code, stdout, errOut := run(t, "capture", "--url", srv.URL, "--out", out)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, errOut)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("capture wrote no file: %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("captured file is not JSON:\n%s", data)
	}
	if !strings.Contains(string(data), "\n  \"openapi\"") {
		t.Errorf("captured JSON is not pretty-printed:\n%s", data)
	}
	if !strings.Contains(stdout, out) {
		t.Errorf("stdout %q does not mention the output path", stdout)
	}

	// Second capture must refuse to overwrite.
	code, _, errOut = run(t, "capture", "--url", srv.URL, "--out", out)
	if code != 2 {
		t.Fatalf("second capture exit = %d, want 2; stderr=%q", code, errOut)
	}
	if !strings.Contains(errOut, "exists") || !strings.Contains(errOut, "--force") {
		t.Errorf("overwrite refusal is not actionable: %q", errOut)
	}

	// --force overwrites.
	code, _, errOut = run(t, "capture", "--url", srv.URL, "--out", out, "--force")
	if code != 0 {
		t.Fatalf("forced capture exit = %d, want 0; stderr=%q", code, errOut)
	}
}

func TestCaptureRejectsHTMLAndWritesNothing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>login</body></html>"))
	}))
	defer srv.Close()

	out := filepath.Join(t.TempDir(), "baseline.json")
	code, _, errOut := run(t, "capture", "--url", srv.URL, "--out", out)
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "is not an OpenAPI document") {
		t.Errorf("stderr = %q, want 'is not an OpenAPI document'", errOut)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("file was written on rejection: %v", err)
	}
	assertNoStackTrace(t, errOut)
}

func TestCaptureNon2xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer srv.Close()

	out := filepath.Join(t.TempDir(), "baseline.json")
	code, _, errOut := run(t, "capture", "--url", srv.URL, "--out", out)
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "401") {
		t.Errorf("stderr does not include the status: %q", errOut)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Error("file was written for a non-2xx response")
	}
	assertNoStackTrace(t, errOut)
}

func TestCaptureMissingFlagsShowsUsage(t *testing.T) {
	code, _, errOut := run(t, "capture", "--url", "http://example.invalid/spec")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "usage: apidiff capture") {
		t.Errorf("stderr does not show capture usage: %q", errOut)
	}
}

func TestUnknownCommandShowsUsage(t *testing.T) {
	code, _, errOut := run(t, "bogus")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "usage: apidiff") {
		t.Errorf("stderr does not show root usage: %q", errOut)
	}
	assertNoStackTrace(t, errOut)
}

func TestNoArgsShowsUsage(t *testing.T) {
	code, _, errOut := run(t)
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "usage: apidiff") {
		t.Errorf("stderr does not show root usage: %q", errOut)
	}
}

func assertNoStackTrace(t *testing.T, output string) {
	t.Helper()
	if strings.Contains(output, "goroutine ") || strings.Contains(output, ".go:") || strings.Contains(output, "panic:") {
		t.Errorf("output looks like a Go stack trace:\n%s", output)
	}
}
