package mdtable

import (
	"strings"
	"testing"
)

// TestProbeCenterAlignPadding verifies that center-aligned cells are padded
// to the exact column width on both sides (left + right padding sum must equal
// the pad amount so that total width matches the column width).
func TestProbeCenterAlignPadding(t *testing.T) {
	// Single column, header "h" (width 1), column width minimum is 3.
	// Center-aligned: content "h" should be padded to width 3 → " h " (1+1+1=3).
	got, widths, err := Format([]string{"h"}, [][]string{{"x"}}, []Alignment{AlignCenter})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if widths[0] != 3 {
		t.Fatalf("column width = %d, want 3", widths[0])
	}
	// Each data/header cell in the output must have display width equal to column width.
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	for idx, line := range lines {
		if idx == 1 {
			continue // skip separator line
		}
		// Extract cell content between "| " and " |".
		inner := strings.TrimPrefix(line, "| ")
		inner = strings.TrimSuffix(inner, " |")
		if displayWidth(inner) != widths[0] {
			t.Errorf("line %d cell width = %d, want %d; line: %q",
				idx, displayWidth(inner), widths[0], line)
		}
	}
}

// TestProbeRightAlignSeparator verifies that the separator cell for a
// right-aligned column ends with a colon (GFM spec: "---:" for right).
func TestProbeRightAlignSeparator(t *testing.T) {
	got, _, err := Format([]string{"Price"}, [][]string{{"9.99"}}, []Alignment{AlignRight})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}
	sep := lines[1]
	// The separator cell content (between pipes) must end with ':'
	inner := strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(sep, " |"), "|"))
	if !strings.HasSuffix(inner, ":") {
		t.Errorf("right-align separator should end with ':', got %q", inner)
	}
	if strings.HasPrefix(inner, ":") {
		t.Errorf("right-align separator should not start with ':', got %q", inner)
	}
}

// TestProbeBackslashEscape verifies that literal backslashes in cell content
// are escaped to "\\" in the rendered output, preserving round-trip fidelity.
func TestProbeBackslashEscape(t *testing.T) {
	// Cell contains a single literal backslash: `a\b`
	got, _, err := Format([]string{"col"}, [][]string{{`a\b`}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// In the rendered table the backslash must be doubled.
	if !strings.Contains(got, `a\\b`) {
		t.Errorf("backslash not escaped in output: %q", got)
	}
}

// TestProbeExplicitDefaultAlign verifies that explicitly passing AlignDefault
// in the alignment slice is accepted without error (it is a valid enum value).
func TestProbeExplicitDefaultAlign(t *testing.T) {
	aligns := []Alignment{AlignDefault, AlignLeft}
	_, _, err := Format([]string{"a", "b"}, [][]string{{"1", "2"}}, aligns)
	if err != nil {
		t.Errorf("explicit AlignDefault should be valid, got error: %v", err)
	}
}

// TestProbeControlCharWidth verifies that C0 control characters (U+0000–U+001F)
// are treated as zero-width for column alignment purposes, since they produce
// no visible glyph in a monospace terminal.
func TestProbeControlCharWidth(t *testing.T) {
	// ESC (0x1B) and other C0 controls should have display width 0.
	cases := []struct {
		r    rune
		desc string
	}{
		{'\x1b', "ESC"},
		{'\x11', "DC1"},
		{'\x1f', "US"},
	}
	for _, c := range cases {
		w := DisplayWidth(string(c.r))
		if w != 0 {
			t.Errorf("DisplayWidth(%s / U+%04X) = %d, want 0", c.desc, c.r, w)
		}
	}
}
