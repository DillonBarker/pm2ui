package ui

import (
	"bytes"
	"testing"
)

func TestTabTitleSetSavesOnce(t *testing.T) {
	var buf bytes.Buffer
	tt := &TabTitle{w: &buf, enabled: true}

	tt.Set("pm2ui ●8")
	tt.Set("pm2ui ●7 ✖api")

	got := buf.String()
	want := "\x1b[22;1t" + "\x1b]1;pm2ui ●8\x07" + "\x1b]1;pm2ui ●7 ✖api\x07"
	if got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestTabTitleRestore(t *testing.T) {
	var buf bytes.Buffer
	tt := &TabTitle{w: &buf, enabled: true}

	tt.Set("pm2ui ●8")
	buf.Reset()
	tt.Restore()

	want := "\x1b]1;\x07" + "\x1b[23;1t"
	if got := buf.String(); got != want {
		t.Errorf("restore output = %q, want %q", got, want)
	}

	buf.Reset()
	tt.Restore() // second restore is a no-op
	if got := buf.String(); got != "" {
		t.Errorf("second restore wrote %q, want nothing", got)
	}
}

func TestTabTitleRestoreWithoutSetIsNoop(t *testing.T) {
	var buf bytes.Buffer
	tt := &TabTitle{w: &buf, enabled: true}

	tt.Restore()

	if got := buf.String(); got != "" {
		t.Errorf("wrote %q, want nothing", got)
	}
}

func TestTabTitleDisabledWritesNothing(t *testing.T) {
	var buf bytes.Buffer
	tt := &TabTitle{w: &buf, enabled: false}

	tt.Set("pm2ui ●8")
	tt.Restore()

	if got := buf.String(); got != "" {
		t.Errorf("wrote %q, want nothing", got)
	}
}
