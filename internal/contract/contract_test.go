package contract

import (
	"os"
	"strings"
	"testing"
)

func readInput(t *testing.T, name string) Input {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return Input{Name: name, Data: data}
}

func mustCompare(t *testing.T, base, rev Input, opts Options) *Result {
	t.Helper()
	res, err := Compare(base, rev, opts)
	if err != nil {
		t.Fatalf("Compare(%s, %s): %v", base.Name, rev.Name, err)
	}
	return res
}

func findChange(changes []Change, id, path string) *Change {
	for i := range changes {
		if changes[i].ID == id && changes[i].Path == path {
			return &changes[i]
		}
	}
	return nil
}

func TestCompareV1V2SplitsBreaking(t *testing.T) {
	res := mustCompare(t, readInput(t, "testdata/v1.yaml"), readInput(t, "testdata/v2.yaml"), Options{})

	removed := findChange(res.Breaking, "api-path-removed-without-deprecation", "/widgets/{id}")
	if removed == nil {
		t.Fatalf("missing api-path-removed for /widgets/{id}; breaking=%+v", res.Breaking)
	}
	if removed.Level != "ERR" || removed.Breaking != true {
		t.Errorf("removed change level/breaking = %q/%v, want ERR/true", removed.Level, removed.Breaking)
	}
	if removed.Method != "get" {
		t.Errorf("removed method = %q, want get", removed.Method)
	}
	if removed.Pointer != "/paths/~1widgets~1{id}/get" {
		t.Errorf("removed pointer = %q", removed.Pointer)
	}

	required := findChange(res.Breaking, "request-property-became-required", "/widgets")
	if required == nil {
		t.Fatalf("missing request-property-became-required for /widgets; breaking=%+v", res.Breaking)
	}
	if !strings.Contains(required.Text, "'color'") {
		t.Errorf("required text = %q, want it to mention 'color'", required.Text)
	}
	if required.Method != "post" {
		t.Errorf("required method = %q, want post", required.Method)
	}
	if required.Pointer != "/paths/~1widgets/post" {
		t.Errorf("required pointer = %q", required.Pointer)
	}

	for _, c := range res.Breaking {
		if !strings.HasPrefix(c.Pointer, "/paths/") {
			t.Errorf("breaking change %s pointer %q is not under /paths/", c.ID, c.Pointer)
		}
	}
}

func TestCompareV1V2AdditionsAreNonBreaking(t *testing.T) {
	res := mustCompare(t, readInput(t, "testdata/v1.yaml"), readInput(t, "testdata/v2.yaml"), Options{})

	added := findChange(res.NonBreaking, "endpoint-added", "/widgets/search")
	if added == nil {
		t.Fatalf("missing endpoint-added for /widgets/search; nonBreaking=%+v", res.NonBreaking)
	}
	if added.Breaking || added.Level != "INFO" {
		t.Errorf("endpoint-added breaking/level = %v/%q, want false/INFO", added.Breaking, added.Level)
	}
	optional := findChange(res.NonBreaking, "new-optional-request-property", "/widgets")
	if optional == nil {
		t.Fatalf("missing new-optional-request-property; nonBreaking=%+v", res.NonBreaking)
	}
}

func TestCompareAdditiveHasNoBreakingEntries(t *testing.T) {
	res := mustCompare(t, readInput(t, "testdata/v1.yaml"), readInput(t, "testdata/additive.yaml"), Options{})
	if len(res.Breaking) != 0 {
		t.Errorf("breaking entries = %d, want 0: %+v", len(res.Breaking), res.Breaking)
	}
	if len(res.NonBreaking) == 0 {
		t.Fatal("expected at least one non-breaking entry")
	}
	if res.Summary.Breaking != 0 || res.Summary.NonBreaking != len(res.NonBreaking) {
		t.Errorf("summary = %+v, want breaking 0 and nonBreaking %d", res.Summary, len(res.NonBreaking))
	}
}

func TestCompareIdenticalHasNoChanges(t *testing.T) {
	res := mustCompare(t, readInput(t, "testdata/v1.yaml"), readInput(t, "testdata/v1.yaml"), Options{})
	if res.Total() != 0 {
		t.Errorf("total changes = %d, want 0: breaking=%+v nonBreaking=%+v", res.Total(), res.Breaking, res.NonBreaking)
	}
}

func TestCompareMalformedNamesDocument(t *testing.T) {
	_, err := Compare(readInput(t, "testdata/v1.yaml"), readInput(t, "testdata/malformed.yaml"), Options{})
	if err == nil {
		t.Fatal("expected an error for a malformed revision")
	}
	if !strings.Contains(err.Error(), "malformed.yaml") {
		t.Errorf("error %q does not name malformed.yaml", err)
	}
}

func TestCompareMalformedBase(t *testing.T) {
	_, err := Compare(readInput(t, "testdata/malformed.yaml"), readInput(t, "testdata/v1.yaml"), Options{})
	if err == nil || !strings.Contains(err.Error(), "malformed.yaml") {
		t.Fatalf("error = %v, want it to name malformed.yaml", err)
	}
}

func TestCompareEmptyBaseReportsAdditions(t *testing.T) {
	res := mustCompare(t, Input{Name: "empty.yaml"}, readInput(t, "testdata/v1.yaml"), Options{})
	if len(res.Breaking) != 0 {
		t.Errorf("empty base breaking = %+v, want none", res.Breaking)
	}
	if findChange(res.NonBreaking, "endpoint-added", "/widgets") == nil {
		t.Errorf("empty base nonBreaking = %+v, want endpoint-added", res.NonBreaking)
	}
}

func TestCompareEmptyRevisionReportsRemovals(t *testing.T) {
	res := mustCompare(t, readInput(t, "testdata/v1.yaml"), Input{Name: "empty.yaml"}, Options{})
	if findChange(res.Breaking, "api-path-removed-without-deprecation", "/widgets/{id}") == nil {
		t.Errorf("empty revision breaking = %+v, want api-path-removed", res.Breaking)
	}
}

func TestCompareStripsANSI(t *testing.T) {
	res := mustCompare(t, readInput(t, "testdata/v1.yaml"), readInput(t, "testdata/v2.yaml"), Options{})
	for _, c := range append(append([]Change{}, res.Breaking...), res.NonBreaking...) {
		if strings.ContainsRune(c.Text, '\x1b') {
			t.Errorf("change %s text contains ANSI: %q", c.ID, c.Text)
		}
	}
}

func TestCompareIsDeterministicallySorted(t *testing.T) {
	base, rev := readInput(t, "testdata/v1.yaml"), readInput(t, "testdata/v2.yaml")
	first := mustCompare(t, base, rev, Options{})
	second := mustCompare(t, base, rev, Options{})
	if len(first.Breaking) != len(second.Breaking) || len(first.NonBreaking) != len(second.NonBreaking) {
		t.Fatal("repeated comparisons returned different change counts")
	}
	for i := range first.Breaking {
		if first.Breaking[i] != second.Breaking[i] {
			t.Fatalf("breaking order differs at %d: %+v vs %+v", i, first.Breaking[i], second.Breaking[i])
		}
	}
	keys := make([]string, 0, len(first.Breaking))
	for _, c := range first.Breaking {
		keys = append(keys, c.Path+"\x00"+c.Method+"\x00"+c.ID+"\x00"+c.Pointer)
	}
	for i := 1; i < len(keys); i++ {
		if keys[i-1] > keys[i] {
			t.Errorf("breaking changes not sorted: %q before %q", keys[i-1], keys[i])
		}
	}
}

func TestCompareIgnoreFiltersRuleIDs(t *testing.T) {
	res := mustCompare(t, readInput(t, "testdata/v1.yaml"), readInput(t, "testdata/additive.yaml"), Options{Ignore: []string{"endpoint-added"}})
	if res.Total() != 0 {
		t.Errorf("total changes after ignore = %d, want 0: %+v", res.Total(), res.NonBreaking)
	}
}

func TestResultFiresAt(t *testing.T) {
	breaking := mustCompare(t, readInput(t, "testdata/v1.yaml"), readInput(t, "testdata/v2.yaml"), Options{})
	breakingOnly := &Result{Breaking: breaking.Breaking}
	cases := []struct {
		name string
		res  *Result
		lvl  string
		want bool
	}{
		{"ERR with ERR breaking", breakingOnly, "ERR", true},
		{"WARN with ERR breaking", breakingOnly, "WARN", true},
		{"INFO with ERR breaking", breakingOnly, "INFO", true},
	}
	for _, tc := range cases {
		got, err := tc.res.FiresAt(tc.lvl)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, got, tc.want)
		}
	}

	additive := mustCompare(t, readInput(t, "testdata/v1.yaml"), readInput(t, "testdata/additive.yaml"), Options{})
	if fired, _ := additive.FiresAt("ERR"); fired {
		t.Error("additive change fired at ERR, want false")
	}
	if fired, _ := additive.FiresAt("WARN"); fired {
		t.Error("additive change fired at WARN, want false")
	}
	if fired, _ := additive.FiresAt("INFO"); !fired {
		t.Error("additive change did not fire at INFO, want true")
	}
	if _, err := additive.FiresAt("NOPE"); err == nil {
		t.Error("invalid level returned no error")
	}
}
