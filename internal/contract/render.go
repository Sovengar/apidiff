package contract

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Render writes the report in one of the supported formats: text, json or
// markdown. Only the report is written; diagnostics belong to the caller's
// stderr.
func Render(w io.Writer, res *Result, format string) error {
	switch format {
	case "", "text":
		return renderText(w, res)
	case "json":
		return renderJSON(w, res)
	case "markdown":
		return renderMarkdown(w, res)
	default:
		return fmt.Errorf("unknown format %q: want text, json or markdown", format)
	}
}

func renderText(w io.Writer, res *Result) error {
	if res.Total() == 0 {
		_, err := io.WriteString(w, "no contract changes\n")
		return err
	}
	if _, err := fmt.Fprintf(w, "%s: %d breaking, %d non-breaking\n",
		res.Base, res.Summary.Breaking, res.Summary.NonBreaking); err != nil {
		return err
	}
	if len(res.Breaking) > 0 {
		if _, err := io.WriteString(w, "\nbreaking\n"); err != nil {
			return err
		}
		if err := writeTextEntries(w, res.Breaking); err != nil {
			return err
		}
	}
	if len(res.NonBreaking) > 0 {
		if _, err := io.WriteString(w, "\nnon-breaking\n"); err != nil {
			return err
		}
		if err := writeTextEntries(w, res.NonBreaking); err != nil {
			return err
		}
	}
	return nil
}

func writeTextEntries(w io.Writer, changes []Change) error {
	for _, c := range changes {
		target := strings.TrimSpace(strings.ToUpper(c.Method) + " " + c.Path)
		line := "  " + c.ID + "  " + target
		if c.Pointer != "" {
			line += "  (" + c.Pointer + ")"
		}
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "      %s\n", c.Text); err != nil {
			return err
		}
	}
	return nil
}

func renderJSON(w io.Writer, res *Result) error {
	// Copy so a nil slice never marshals as null.
	out := *res
	if out.Breaking == nil {
		out.Breaking = []Change{}
	}
	if out.NonBreaking == nil {
		out.NonBreaking = []Change{}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("encode report: %w", err)
	}
	_, err = w.Write(append(data, '\n'))
	return err
}

func renderMarkdown(w io.Writer, res *Result) error {
	if res.Total() == 0 {
		_, err := io.WriteString(w, "## API contract\n\nno contract changes\n")
		return err
	}
	if _, err := fmt.Fprintf(w, "## API contract\n\n`%s` → `%s`\n\n**%d breaking**, %d non-breaking\n",
		res.Base, res.Revision, res.Summary.Breaking, res.Summary.NonBreaking); err != nil {
		return err
	}
	for _, c := range res.Breaking {
		if _, err := fmt.Fprintf(w, "\n- `%s` `%s` — %s", c.ID, c.Pointer, c.Text); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "\n")
	return err
}
