package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode"
)

// Keep the original algorithm as an independent oracle and profiling baseline.
func referenceReplace(text string, replacements []Replacement) string {
	if len(replacements) == 0 {
		return text
	}
	pairs := append([]Replacement(nil), replacements...)
	sort.SliceStable(pairs, func(i, j int) bool { return len(pairs[i].From) > len(pairs[j].From) })
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = regexp.QuoteMeta(p.From)
	}
	re := regexp.MustCompile("(?i)(" + strings.Join(parts, "|") + ")")
	return re.ReplaceAllStringFunc(text, func(match string) string {
		for _, p := range pairs {
			if strings.EqualFold(match, p.From) {
				return preserveCase(match, p.To)
			}
		}
		return preserveCase(match, "")
	})
}

func TestReplacer(t *testing.T) {
	cases := []struct {
		text  string
		pairs []Replacement
	}{
		{"FooBar fooBar FOOBAR foobar fOObAr フー", []Replacement{{"FooBar", "AaaBbb"}}},
		{"foo-bar FOO-bar Foo-BAR foo_bar FOO_BAR", []Replacement{{"foo-bar", "baz-Qux"}, {"foo_bar", "baz_Qux"}}},
		{"foo-bar FOO-bar Foo-BAR", []Replacement{{"foo-bar", "baz-Qux"}}},
		{"Kelvin KELVIN kelvin ſkill SKILL skill", []Replacement{{"skill", "NewWord"}}},
		{"Kelvin KELVIN kelvin", []Replacement{{"kelvin", "NewWord"}}},
		{"フー ΑΒ αβ Σ σ ς", []Replacement{{"フー", "バー"}, {"Σ", "Ab"}, {"αβ", "Cd"}}},
		{"foobar Foo FOO foo", []Replacement{{"foo", "X"}, {"foobar", "LongWord"}, {"FOO", "Other"}}},
		{"a a.a [a] $a", []Replacement{{"a.a", "$X"}}},
		{"abc 日本語", []Replacement{{"", "X"}}},
		{"aa a 日本語", []Replacement{{"a", ""}, {"", "X"}}},
		{"aaaAa", []Replacement{{"aa", "a"}}},
		{"foo foo", []Replacement{{"foo", "foobar"}}},
		{"\xfffoo\xc0FOO\x00foo", []Replacement{{"foo", "Bar"}}},
		{"foo\x00FOO", []Replacement{{"\x00", "X"}}},
		{"nothing matches フー", []Replacement{{"foo", "Bar"}}},
		{"unchanged", nil},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			r := newReplacer(tc.pairs)
			want := referenceReplace(tc.text, tc.pairs)
			for n := 0; n < 2; n++ {
				if got := r.apply(tc.text); got != want {
					t.Fatalf("got %q; want %q", got, want)
				}
			}
		})
	}
}

func TestCacheBound(t *testing.T) {
	r := newReplacer([]Replacement{{"aaaaaaaaaa", "Bb"}})
	for i := 0; i < 1024; i++ {
		b := []byte("aaaaaaaaaa")
		for j := range b {
			if i&(1<<j) != 0 {
				b[j] = 'A'
			}
		}
		text := string(b)
		if got, want := r.apply(text), referenceReplace(text, r.pairs); got != want {
			t.Fatalf("%q: %q != %q", text, got, want)
		}
	}
	if len(r.cache) > 256 {
		t.Fatal("unbounded cache")
	}
}

func FuzzReplacer(f *testing.F) {
	f.Add("FooBar fooBar フー", "FooBar", "AaaBbb")
	f.Add("ſKsK", "sk", "Hello")
	f.Add("aaa", "", "x")
	f.Fuzz(func(t *testing.T, text, from, to string) {
		// regexp construction requires valid UTF-8 rules, as in the original.
		from = strings.ToValidUTF8(from, "�")
		pairs := []Replacement{{from, to}}
		if got, want := newReplacer(pairs).apply(text), referenceReplace(text, pairs); got != want {
			t.Fatalf("got %q; want %q", got, want)
		}
	})
}

var benchmarkResult string

func BenchmarkReplace(b *testing.B) {
	chunk := "This is file 0.\nWe have FooBar here.\nAlso fooBar and FOOBAR and foobar.\nLet's test Japanese フー.\nAnd some other text barBar.\n"
	for _, tc := range []struct {
		name, text string
		pairs      []Replacement
	}{
		{"README", strings.Repeat(chunk, 10000), []Replacement{{"FooBar", "AaaBbb"}}},
		{"NoMatch", strings.Repeat("Nothing to replace 日本語.\n", 10000), []Replacement{{"FooBar", "AaaBbb"}}},
		{"Multiple", strings.Repeat(chunk, 10000), []Replacement{{"FooBar", "AaaBbb"}, {"フー", "バー"}, {"barBar", "cccDdd"}}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.Run("Before", func(b *testing.B) {
				b.SetBytes(int64(len(tc.text)))
				b.ReportAllocs()
				for b.Loop() {
					benchmarkResult = referenceReplace(tc.text, tc.pairs)
				}
			})
			b.Run("After", func(b *testing.B) {
				r := newReplacer(tc.pairs)
				b.SetBytes(int64(len(tc.text)))
				b.ReportAllocs()
				for b.Loop() {
					benchmarkResult = r.apply(tc.text)
				}
			})
		})
	}
}

func TestASCIIFoldClasses(t *testing.T) {
	for c := rune(0); c < 128; c++ {
		want := byte(c)
		if want >= 'A' && want <= 'Z' {
			want += 'a' - 'A'
		}
		for r := unicode.SimpleFold(c); r != c; r = unicode.SimpleFold(r) {
			text := string(r)
			got, size := foldASCII(text, 0)
			if got != want || size != len(text) {
				t.Fatalf("fold %U: %x/%d, want %x/%d", r, got, size, want, len(text))
			}
		}
	}
}

func TestLongLiteral(t *testing.T) {
	from := strings.Repeat("a", 65)
	pairs := []Replacement{{from, "Bb"}}
	r := newReplacer(pairs)
	if r.re == nil {
		t.Fatal("long literal must use regexp to bound rescanning")
	}
	text := strings.Repeat("a", 200)
	if got, want := r.apply(text), referenceReplace(text, pairs); got != want {
		t.Fatalf("got %q; want %q", got, want)
	}
}
