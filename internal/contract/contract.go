// Package contract loads two OpenAPI documents, diffs them with oasdiff and
// classifies every finding as breaking or non-breaking.
package contract

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/oasdiff/oasdiff/checker"
	"github.com/oasdiff/oasdiff/diff"
	"github.com/oasdiff/oasdiff/load"
)

// Input is one side of a comparison: the caller-provided name (used in the
// report and in errors) and the raw document bytes.
type Input struct {
	Name string
	Data []byte
}

// Options controls what Compare reports.
type Options struct {
	// Ignore is a set of rule ids to drop from the result.
	Ignore []string
}

// Change is one classified finding.
type Change struct {
	ID       string `json:"id"`
	Level    string `json:"level"`
	Path     string `json:"path"`
	Method   string `json:"method"`
	Pointer  string `json:"pointer"`
	Text     string `json:"text"`
	Breaking bool   `json:"-"`

	level checker.Level
}

// Summary counts the findings by class.
type Summary struct {
	Breaking    int `json:"breaking"`
	NonBreaking int `json:"nonBreaking"`
}

// Result is the full comparison report. Base and Revision echo the names the
// caller passed, not resolved paths.
type Result struct {
	Base        string   `json:"base"`
	Revision    string   `json:"revision"`
	Breaking    []Change `json:"breaking"`
	NonBreaking []Change `json:"nonBreaking"`
	Summary     Summary  `json:"summary"`
}

// Total returns the number of reported changes.
func (r *Result) Total() int {
	return len(r.Breaking) + len(r.NonBreaking)
}

// FiresAt reports whether any finding is at or above the given level. The
// level string is the same vocabulary as --fail-on: ERR, WARN or INFO.
func (r *Result) FiresAt(level string) (bool, error) {
	target, err := checker.NewLevel(level)
	if err != nil {
		return false, fmt.Errorf("invalid level %q: %w", level, err)
	}
	switch target {
	case checker.ERR, checker.WARN, checker.INFO:
	default:
		return false, fmt.Errorf("invalid level %q: want ERR, WARN or INFO", level)
	}
	for _, c := range append(append([]Change{}, r.Breaking...), r.NonBreaking...) {
		if c.level >= target {
			return true, nil
		}
	}
	return false, nil
}

// Compare loads both documents with a fresh loader each (the loader caches by
// name) and returns the classified findings.
func Compare(base, rev Input, opts Options) (*Result, error) {
	baseSpec, err := loadSpec(base)
	if err != nil {
		return nil, err
	}
	revSpec, err := loadSpec(rev)
	if err != nil {
		return nil, err
	}

	report, sources, err := diff.GetWithOperationsSourcesMap(diff.NewConfig(), baseSpec, revSpec)
	if err != nil {
		return nil, fmt.Errorf("compare %s with %s: %w", base.Name, rev.Name, err)
	}

	// CheckBackwardCompatibility filters to WARN; UntilLevel(INFO) keeps the
	// additive findings too.
	cfg := checker.NewConfig(checker.GetAllChecks())
	changes := checker.CheckBackwardCompatibilityUntilLevel(cfg, report, sources, checker.INFO)

	ignored := make(map[string]bool, len(opts.Ignore))
	for _, id := range opts.Ignore {
		if id = strings.TrimSpace(id); id != "" {
			ignored[id] = true
		}
	}

	localizer := checker.NewDefaultLocalizer()
	res := &Result{
		Base:        base.Name,
		Revision:    rev.Name,
		Breaking:    []Change{},
		NonBreaking: []Change{},
	}
	for _, c := range changes {
		if ignored[c.GetId()] {
			continue
		}
		ch := toChange(c, localizer)
		if ch.Breaking {
			res.Breaking = append(res.Breaking, ch)
		} else {
			res.NonBreaking = append(res.NonBreaking, ch)
		}
	}
	sortChanges(res.Breaking)
	sortChanges(res.NonBreaking)
	res.Summary = Summary{Breaking: len(res.Breaking), NonBreaking: len(res.NonBreaking)}
	return res, nil
}

func loadSpec(in Input) (*load.SpecInfo, error) {
	// Never share a loader: it caches documents by name and both sides may use
	// the same name.
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	spec, err := load.NewSpecInfoFromData(loader, in.Data, in.Name)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", in.Name, err)
	}
	return spec, nil
}

func toChange(c checker.Change, localizer checker.Localizer) Change {
	return Change{
		ID:       c.GetId(),
		Level:    levelName(c.GetLevel()),
		Path:     c.GetPath(),
		Method:   strings.ToLower(c.GetOperation()),
		Pointer:  pointer(c.GetPath(), c.GetOperation()),
		Text:     ansi.Strip(c.GetText(localizer)),
		Breaking: c.IsBreaking(),
		level:    c.GetLevel(),
	}
}

func levelName(level checker.Level) string {
	switch level {
	case checker.ERR:
		return "ERR"
	case checker.WARN:
		return "WARN"
	case checker.INFO:
		return "INFO"
	default:
		return "INFO"
	}
}

// pointer builds a JSON pointer under /paths/ for the finding, escaping the
// path segment per RFC 6901. Findings without a path (such as a removed
// component schema) have no pointer.
func pointer(path, method string) string {
	if path == "" {
		return ""
	}
	escaped := strings.NewReplacer("~", "~0", "/", "~1").Replace(path)
	p := "/paths/" + escaped
	if method = strings.ToLower(method); method != "" {
		p += "/" + method
	}
	return p
}

func sortChanges(changes []Change) {
	sort.SliceStable(changes, func(i, j int) bool {
		a, b := changes[i], changes[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Method != b.Method {
			return a.Method < b.Method
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		return a.Pointer < b.Pointer
	})
}
