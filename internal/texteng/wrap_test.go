package texteng

import "testing"

// measure1 is a deterministic fake: every rune is 1px, space is 1px.
func measure1(s string) (float64, error) { return float64(len([]rune(s))), nil }

func TestWrapNone(t *testing.T) {
	lines := Wrap("aaa bbb ccc\nnext", 10, "none", measure1)
	if len(lines) != 2 || lines[0].text != "aaa bbb ccc" || lines[1].text != "next" {
		t.Fatalf("got %#v", lines)
	}
}

func TestWrapWord(t *testing.T) {
	lines := Wrap("aa bb cc dd ee", 8, "word", measure1)
	// widths: aa=2, bb=2, cc=2, dd=2, ee=2; space=1
	// "aa bb cc"=8 fits, + " dd"=3 -> 11 > 8
	want := []string{"aa bb cc", "dd ee"}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines %#v, want %d", len(lines), lines, len(want))
	}
	for i := range want {
		if lines[i].text != want[i] {
			t.Fatalf("line %d = %q, want %q", i, lines[i].text, want[i])
		}
	}
}

func TestWrapWordOverflowEmitsWholeWord(t *testing.T) {
	lines := Wrap("short verylongword tail", 10, "word", measure1)
	// verylongword (12) > 10: must be emitted whole (overflow reported later)
	found := false
	for _, l := range lines {
		if l.text == "verylongword" {
			found = true
		}
		if l.width > 10 && l.text != "verylongword" {
			t.Fatalf("line %q wider than max without being the long word", l.text)
		}
	}
	if !found {
		t.Fatalf("long word lost: %#v", lines)
	}
}

func TestWrapWordChar(t *testing.T) {
	lines := Wrap("short verylongword tail", 10, "word_char", measure1)
	for _, l := range lines {
		if l.width > 10 {
			t.Fatalf("line %q (%.0f) exceeds max 10", l.text, l.width)
		}
	}
	// all content preserved (spaces dropped at line breaks)
	total := ""
	for _, l := range lines {
		total += l.text
	}
	if want := "shortverylongwordtail"; total != want {
		t.Fatalf("content mangled: %q, want %q", total, want)
	}
}

func TestWrapChar(t *testing.T) {
	lines := Wrap("abcdefghij", 4, "char", measure1)
	want := []string{"abcd", "efgh", "ij"}
	if len(lines) != len(want) {
		t.Fatalf("got %#v", lines)
	}
	for i := range want {
		if lines[i].text != want[i] {
			t.Fatalf("line %d = %q, want %q", i, lines[i].text, want[i])
		}
	}
}

func TestWrapEmptyAndSpaces(t *testing.T) {
	if lines := Wrap("", 10, "word_char", measure1); len(lines) != 0 {
		t.Fatalf("empty: %#v", lines)
	}
	lines := Wrap("   a  b   ", 100, "word_char", measure1)
	if len(lines) != 1 || lines[0].text != "a b" {
		t.Fatalf("spaces: %#v", lines)
	}
	// explicit blank line is preserved as an empty line
	lines = Wrap("ab\n\ncd", 100, "word_char", measure1)
	if len(lines) != 3 || lines[0].text != "ab" || lines[1].text != "" || lines[2].text != "cd" {
		t.Fatalf("blank line: %#v", lines)
	}
}

func TestWrapNewlineAndLongWord(t *testing.T) {
	lines := Wrap("ab\ncdefghij", 5, "word_char", measure1)
	// para1 "ab" -> "ab"; para2 "cdefghij" (8>5) -> "cdefg","hij"
	if len(lines) != 3 || lines[0].text != "ab" || lines[1].text != "cdefg" || lines[2].text != "hij" {
		t.Fatalf("got %#v", lines)
	}
}
