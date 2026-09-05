package texteng

import (
	"os/exec"
	"strings"
	"testing"
)

func TestFamilyNamesMatching(t *testing.T) {
	f := fontEntry{Family: "Go Regular:Sans,Condensed"}
	for _, want := range []string{"go regular", "sans", "Condensed"} {
		if !familyMatches(f, want) {
			t.Fatalf("familyMatches(%q) = false, want true", want)
		}
	}
	if familyMatches(f, "Serif") {
		t.Fatal("familyMatches(Serif) = true, want false")
	}
	if got := familyNames(" A :B, C "); len(got) != 3 || got[0] != "A" || got[2] != "C" {
		t.Fatalf("familyNames: %#v", got)
	}
}

var testFonts = []fontEntry{
	{Family: "Sans,Condensed", Style: "Regular", File: "/fonts/sans-r.ttf"},
	{Family: "Sans,Condensed", Style: "Bold", File: "/fonts/sans-b.ttf"},
	{Family: "Serif:Old Serif", Style: "Italic", File: "/fonts/serif-i.otf"},
	// A condensed face that advertises itself under the "Sans" alias:
	// it must never win against the primary-family entry, whatever the
	// database order.
	{Family: "Sans Condensed:Sans", Style: "Bold", File: "/fonts/sans-cond-b.ttf"},
}

func TestMatchFontPrimaryFamilyBeatsAlias(t *testing.T) {
	f, err := matchFont(testFonts, "Sans", "Bold")
	if err != nil {
		t.Fatal(err)
	}
	if f.File != "/fonts/sans-b.ttf" {
		t.Fatalf("got %q, want the primary-family face", f.File)
	}
	// The alias is still reachable when the primary is absent.
	only := []fontEntry{testFonts[3]}
	f, err = matchFont(only, "Sans", "Bold")
	if err != nil {
		t.Fatal(err)
	}
	if f.File != "/fonts/sans-cond-b.ttf" {
		t.Fatalf("got %q", f.File)
	}
}

func TestMatchFontByFamilyElement(t *testing.T) {
	// "Old Serif" is only the second name of the entry; whole-string
	// comparison of the family column would miss it.
	f, err := matchFont(testFonts, "Old Serif", "")
	if err != nil {
		t.Fatal(err)
	}
	if f.File != "/fonts/serif-i.otf" {
		t.Fatalf("got %q", f.File)
	}
}

func TestMatchFontByStyle(t *testing.T) {
	f, err := matchFont(testFonts, "sans", "bold")
	if err != nil {
		t.Fatal(err)
	}
	if f.File != "/fonts/sans-b.ttf" {
		t.Fatalf("got %q", f.File)
	}
}

func TestMatchFontStyleRejected(t *testing.T) {
	_, err := matchFont(testFonts, "Sans", "Light")
	if err == nil {
		t.Fatal("want style rejection")
	}
	for _, want := range []string{"available styles", "Regular", "Bold"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q lacks %q", err, want)
		}
	}
}

func TestMatchFontFamilyRejected(t *testing.T) {
	// Pattern syntax in the value must be inert: it is an opaque string.
	_, err := matchFont(testFonts, "Go=Regular:style=Bold", "")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("want family-not-found, got %v", err)
	}
}

func skipNoFontconfig(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("fc-list"); err != nil {
		t.Skip("fontconfig not installed")
	}
}

// TestResolvePathEndToEnd covers the real fc-list path once: the first
// advertised family must be resolvable to a file.
func TestResolvePathEndToEnd(t *testing.T) {
	skipNoFontconfig(t)
	fonts, err := listFonts()
	if err != nil {
		t.Fatal(err)
	}
	if len(fonts) == 0 {
		t.Fatal("no fonts listed")
	}
	fam := familyNames(fonts[0].Family)
	if len(fam) == 0 {
		t.Fatal("no family name in first entry")
	}
	got, err := resolvePath(Options{Family: fam[0], Style: fonts[0].Style, SizePx: 16})
	if err != nil {
		t.Fatalf("family %q style %q: %v", fam[0], fonts[0].Style, err)
	}
	if got == "" {
		t.Fatal("empty file path")
	}
	if _, err := resolvePath(Options{Family: "definitely-not-a-real-family-xyz", SizePx: 16}); err == nil {
		t.Fatal("want error for unknown family")
	}
}
