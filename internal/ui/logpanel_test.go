package ui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

func TestHighlightMatches_Single(t *testing.T) {
	re := regexp.MustCompile("boom")
	got := highlightMatches("error: boom happened", re)
	want := "error: [black:yellow]boom[-:-] happened"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHighlightMatches_Multiple(t *testing.T) {
	re := regexp.MustCompile("a")
	got := highlightMatches("banana", re)
	want := "b[black:yellow]a[-:-]n[black:yellow]a[-:-]n[black:yellow]a[-:-]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHighlightMatches_NoMatch(t *testing.T) {
	re := regexp.MustCompile("zzz")
	got := highlightMatches("hello world", re)
	if got != "hello world" {
		t.Errorf("got %q, want unchanged", got)
	}
}

func TestHighlightMatches_EscapesBrackets(t *testing.T) {
	re := regexp.MustCompile("GET")
	got := highlightMatches("[api] GET /health", re)
	// The non-matching "[api]" segment must be escaped so tview doesn't eat
	// it as a color tag.
	if got == "[api] [black:yellow]GET[-:-] /health" {
		t.Errorf("bracket segment not escaped: %q", got)
	}
	if want := "[black:yellow]GET[-:-]"; !regexp.MustCompile(regexp.QuoteMeta(want)).MatchString(got) {
		t.Errorf("match not highlighted: %q", got)
	}
}

func TestHighlightMatches_CaseInsensitivePattern(t *testing.T) {
	re := regexp.MustCompile("(?i)error")
	got := highlightMatches("ERROR: bad", re)
	want := "[black:yellow]ERROR[-:-]: bad"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func testEntries() []LogEntry {
	return []LogEntry{
		{Display: "api line 1", Plain: "api line 1", Process: "api"},
		{Display: "web line 1", Plain: "web line 1", Process: "web"},
		{Display: "api line 2", Plain: "api line 2", Process: "api"},
		{Display: "worker line 1", Plain: "worker line 1", Process: "worker"},
	}
}

func TestServiceFilter_ShowsOnlySelected(t *testing.T) {
	lp := NewLogPanel()
	lp.AppendLines(testEntries())

	lp.SetServiceFilter([]string{"api"})
	text := lp.GetText(true)
	if !strings.Contains(text, "api line 1") || !strings.Contains(text, "api line 2") {
		t.Errorf("api lines missing: %q", text)
	}
	if strings.Contains(text, "web line 1") || strings.Contains(text, "worker line 1") {
		t.Errorf("unselected services leaked: %q", text)
	}
}

func TestServiceFilter_ClearRestoresBuffer(t *testing.T) {
	lp := NewLogPanel()
	lp.AppendLines(testEntries())

	lp.SetServiceFilter([]string{"web"})
	lp.SetServiceFilter(nil)
	text := lp.GetText(true)
	for _, want := range []string{"api line 1", "web line 1", "api line 2", "worker line 1"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q after clearing filter: %q", want, text)
		}
	}
}

func TestServiceFilter_AppliesToIncomingLines(t *testing.T) {
	lp := NewLogPanel()
	lp.SetServiceFilter([]string{"api"})
	lp.AppendLines(testEntries())

	text := lp.GetText(true)
	if !strings.Contains(text, "api line 1") {
		t.Errorf("api line missing: %q", text)
	}
	if strings.Contains(text, "web line 1") {
		t.Errorf("filtered service leaked on append: %q", text)
	}
	// Buffer keeps everything so widening the filter recovers hidden lines.
	lp.SetServiceFilter([]string{"api", "web"})
	if text := lp.GetText(true); !strings.Contains(text, "web line 1") {
		t.Errorf("web line lost from buffer: %q", text)
	}
}

func TestMarkEntry_BypassesServiceFilterAndSearch(t *testing.T) {
	lp := NewLogPanel()
	lp.AppendLines(testEntries())
	lp.AppendLines([]LogEntry{{Display: "── mark ──", Plain: "── mark ──", Mark: true}})

	lp.SetServiceFilter([]string{"api"})
	lp.SetSearch(regexp.MustCompile("line 1"))
	text := lp.GetText(true)
	if !strings.Contains(text, "── mark ──") {
		t.Errorf("mark hidden by filters: %q", text)
	}
}

func TestClearContent_KeepsSearchAndFilter(t *testing.T) {
	lp := NewLogPanel()
	lp.AppendLines(testEntries())
	lp.SetServiceFilter([]string{"api"})
	lp.SetSearch(regexp.MustCompile("line"))

	lp.ClearContent()
	if len(lp.Buffer()) != 0 {
		t.Errorf("buffer not cleared: %d entries", len(lp.Buffer()))
	}
	if !lp.HasSearch() {
		t.Error("search lost on ClearContent")
	}

	// New lines still honor the surviving filters.
	lp.AppendLines([]LogEntry{
		{Display: "api line 9", Plain: "api line 9", Process: "api"},
		{Display: "web line 9", Plain: "web line 9", Process: "web"},
	})
	text := lp.GetText(true)
	if !strings.Contains(text, "api line 9") {
		t.Errorf("matching line missing after clear: %q", text)
	}
	if strings.Contains(text, "web line 9") {
		t.Errorf("service filter lost after clear: %q", text)
	}
}

func TestBufferEviction_FairShareAcrossServices(t *testing.T) {
	lp := NewLogPanel()
	lp.SetMaxLines(10)

	// A quiet service's old lines must survive a later flood from a noisy
	// one — the exact all-services-startup scenario.
	var entries []LogEntry
	for range 3 {
		entries = append(entries, LogEntry{Plain: "quiet", Display: "quiet", Process: "quiet"})
	}
	for range 20 {
		entries = append(entries, LogEntry{Plain: "noisy", Display: "noisy", Process: "noisy"})
	}
	lp.AppendLines(entries)

	if len(lp.Buffer()) > 10 {
		t.Errorf("buffer over cap: %d", len(lp.Buffer()))
	}
	quiet := 0
	for _, e := range lp.Buffer() {
		if e.Process == "quiet" {
			quiet++
		}
	}
	if quiet != 3 {
		t.Errorf("quiet service lost history: kept %d of 3 lines", quiet)
	}
}

func TestBufferEviction_SingleServiceKeepsNewest(t *testing.T) {
	lp := NewLogPanel()
	lp.SetMaxLines(5)

	var entries []LogEntry
	for i := range 8 {
		entries = append(entries, LogEntry{
			Plain:   fmt.Sprintf("line-%d", i),
			Display: fmt.Sprintf("line-%d", i),
		})
	}
	lp.AppendLines(entries)

	buf := lp.Buffer()
	if len(buf) != 5 {
		t.Fatalf("buffer = %d entries, want 5", len(buf))
	}
	if buf[0].Plain != "line-3" || buf[4].Plain != "line-7" {
		t.Errorf("wrong entries kept: first=%q last=%q", buf[0].Plain, buf[4].Plain)
	}
}

func TestServiceFilter_CombinesWithSearch(t *testing.T) {
	lp := NewLogPanel()
	lp.AppendLines(testEntries())

	lp.SetServiceFilter([]string{"api", "web"})
	lp.SetSearch(regexp.MustCompile("line 1"))
	text := lp.GetText(true)
	if !strings.Contains(text, "api line 1") || !strings.Contains(text, "web line 1") {
		t.Errorf("matching lines missing: %q", text)
	}
	if strings.Contains(text, "api line 2") || strings.Contains(text, "worker") {
		t.Errorf("filter/search combination leaked: %q", text)
	}
}
