package contract

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func v1v2(t *testing.T) *Result {
	t.Helper()
	return mustCompare(t, readInput(t, "testdata/v1.yaml"), readInput(t, "testdata/v2.yaml"), Options{})
}

func render(t *testing.T, res *Result, format string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Render(&buf, res, format); err != nil {
		t.Fatalf("Render(%q): %v", format, err)
	}
	return buf.String()
}

func TestRenderText(t *testing.T) {
	out := render(t, v1v2(t), "text")
	for _, want := range []string{
		"2 breaking, 2 non-breaking",
		"breaking",
		"api-path-removed-without-deprecation",
		"GET /widgets/{id}",
		"(/paths/~1widgets~1{id}/get)",
		"API path removed without deprecation",
		"non-breaking",
		"endpoint-added",
		"/widgets/search",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q:\n%s", want, out)
		}
	}
	if strings.ContainsRune(out, '\x1b') {
		t.Errorf("text output contains ANSI: %q", out)
	}
	if strings.Index(out, "breaking") > strings.Index(out, "non-breaking") {
		t.Error("breaking section should come before non-breaking")
	}
}

func TestRenderTextNoChanges(t *testing.T) {
	identical := mustCompare(t, readInput(t, "testdata/v1.yaml"), readInput(t, "testdata/v1.yaml"), Options{})
	out := render(t, identical, "text")
	if strings.TrimSpace(out) != "no contract changes" {
		t.Errorf("text output = %q, want exactly one 'no contract changes' line", out)
	}
}

func TestRenderTextEmptyBreakingSectionOmitted(t *testing.T) {
	additive := mustCompare(t, readInput(t, "testdata/v1.yaml"), readInput(t, "testdata/additive.yaml"), Options{})
	out := render(t, additive, "text")
	if !strings.Contains(out, "0 breaking, 1 non-breaking") {
		t.Errorf("text output missing summary:\n%s", out)
	}
	if strings.Contains(out, "\nbreaking\n") {
		t.Errorf("empty breaking section should be omitted:\n%s", out)
	}
}

func TestRenderJSON(t *testing.T) {
	out := render(t, v1v2(t), "json")
	if strings.ContainsRune(out, '\x1b') {
		t.Fatalf("JSON output contains ANSI: %q", out)
	}
	var doc struct {
		Base        string   `json:"base"`
		Revision    string   `json:"revision"`
		Breaking    []Change `json:"breaking"`
		NonBreaking []Change `json:"nonBreaking"`
		Summary     struct {
			Breaking    int `json:"breaking"`
			NonBreaking int `json:"nonBreaking"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("output is not one JSON document: %v\n%s", err, out)
	}
	if doc.Base != "testdata/v1.yaml" || doc.Revision != "testdata/v2.yaml" {
		t.Errorf("base/revision = %q/%q, want the caller-provided names", doc.Base, doc.Revision)
	}
	if len(doc.Breaking) != 2 || len(doc.NonBreaking) != 2 {
		t.Errorf("breaking/nonBreaking = %d/%d, want 2/2", len(doc.Breaking), len(doc.NonBreaking))
	}
	if doc.Summary.Breaking != 2 || doc.Summary.NonBreaking != 2 {
		t.Errorf("summary = %+v, want 2/2", doc.Summary)
	}
	for _, c := range append(doc.Breaking, doc.NonBreaking...) {
		if c.ID == "" || c.Level == "" || c.Pointer == "" || c.Text == "" {
			t.Errorf("incomplete change in JSON: %+v", c)
		}
	}
}

func TestRenderJSONEmptyListsAreArrays(t *testing.T) {
	identical := mustCompare(t, readInput(t, "testdata/v1.yaml"), readInput(t, "testdata/v1.yaml"), Options{})
	out := render(t, identical, "json")
	if !strings.Contains(out, `"breaking": []`) || !strings.Contains(out, `"nonBreaking": []`) {
		t.Errorf("empty lists should be [] not null:\n%s", out)
	}
}

func TestRenderMarkdown(t *testing.T) {
	out := render(t, v1v2(t), "markdown")
	for _, want := range []string{
		"## API contract",
		"`testdata/v1.yaml` → `testdata/v2.yaml`",
		"**2 breaking**, 2 non-breaking",
		"`api-path-removed-without-deprecation` `/paths/~1widgets~1{id}/get`",
		"`request-property-became-required` `/paths/~1widgets/post`",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "endpoint-added") || strings.Contains(out, "new-optional-request-property") {
		t.Errorf("markdown should not list non-breaking entries individually:\n%s", out)
	}
	if strings.ContainsRune(out, '\x1b') {
		t.Errorf("markdown output contains ANSI: %q", out)
	}
}

func TestRenderRejectsUnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, v1v2(t), "yaml"); err == nil {
		t.Fatal("expected an error for an unknown format")
	}
}
