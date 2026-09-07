package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

func preserveCase(match, toStr string) string {
	if toStr == "" {
		return ""
	}

	if strings.Contains(match, "-") && strings.Contains(toStr, "-") {
		mParts := strings.Split(match, "-")
		tParts := strings.Split(toStr, "-")
		if len(mParts) == len(tParts) {
			var res []string
			for i := 0; i < len(mParts); i++ {
				res = append(res, preserveCase(mParts[i], tParts[i]))
			}
			return strings.Join(res, "-")
		}
	}

	if strings.Contains(match, "_") && strings.Contains(toStr, "_") {
		mParts := strings.Split(match, "_")
		tParts := strings.Split(toStr, "_")
		if len(mParts) == len(tParts) {
			var res []string
			for i := 0; i < len(mParts); i++ {
				res = append(res, preserveCase(mParts[i], tParts[i]))
			}
			return strings.Join(res, "_")
		}
	}

	matchUpper := strings.ToUpper(match)
	matchLower := strings.ToLower(match)

	if match == matchUpper {
		return strings.ToUpper(toStr)
	} else if match == matchLower {
		return strings.ToLower(toStr)
	} else {
		matchRunes := []rune(match)
		toRunes := []rune(toStr)
		if len(matchRunes) > 0 && len(toRunes) > 0 {
			firstMatch := string(matchRunes[0])
			firstTo := string(toRunes[0])
			restTo := string(toRunes[1:])

			if firstMatch == strings.ToLower(firstMatch) {
				return strings.ToLower(firstTo) + restTo
			} else {
				return strings.ToUpper(firstTo) + restTo
			}
		}
	}
	return toStr
}

type Replacement struct {
	From string
	To   string
}

// Replacer compiles rules once and reuses case conversions across files.
// It is used sequentially by the copy loop.
type Replacer struct {
	pairs   []Replacement
	re      *regexp.Regexp
	literal string
	cache   map[string]string
}

func newReplacer(replacements []Replacement) *Replacer {
	r := &Replacer{pairs: append([]Replacement(nil), replacements...), cache: make(map[string]string)}
	if len(r.pairs) == 0 {
		return r
	}
	sort.SliceStable(r.pairs, func(i, j int) bool { return len(r.pairs[i].From) > len(r.pairs[j].From) })
	// Short ASCII literals need no regular-expression engine. Bound their
	// length so repeated partial matches cannot cause unbounded rescanning.
	if len(r.pairs) == 1 && len(r.pairs[0].From) > 0 && len(r.pairs[0].From) <= 64 {
		ascii := true
		for _, c := range r.pairs[0].From {
			if c >= utf8.RuneSelf {
				ascii = false
				break
			}
		}
		if ascii {
			r.literal = strings.ToLower(r.pairs[0].From)
			return r
		}
	}
	parts := make([]string, len(r.pairs))
	for i, p := range r.pairs {
		parts[i] = regexp.QuoteMeta(p.From)
	}
	r.re = regexp.MustCompile("(?i)(" + strings.Join(parts, "|") + ")")
	return r
}

func (r *Replacer) replacement(match string) string {
	if to, ok := r.cache[match]; ok {
		return to
	}
	var to string
	for _, p := range r.pairs {
		if strings.EqualFold(match, p.From) {
			to = preserveCase(match, p.To)
			break
		}
	}
	// Bound retained input and case variants, including for long user rules.
	if len(r.cache) < 256 && len(match) <= 256 {
		r.cache[strings.Clone(match)] = to
	}
	return to
}

// foldASCII also accepts the two non-ASCII runes in ASCII Unicode simple-fold
// classes, so literals containing K or S keep regexp's Unicode semantics.
func foldASCII(text string, pos int) (byte, int) {
	c := text[pos]
	if c < utf8.RuneSelf {
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		return c, 1
	}
	ch, size := utf8.DecodeRuneInString(text[pos:])
	switch ch {
	case 'K':
		return 'k', size
	case 'ſ':
		return 's', size
	}
	return 0xff, size
}

func (r *Replacer) apply(text string) string {
	if len(r.pairs) == 0 {
		return text
	}
	if r.literal == "" {
		return r.re.ReplaceAllStringFunc(text, r.replacement)
	}
	var out strings.Builder
	last := 0
	for pos := 0; pos < len(text); {
		c, size := foldASCII(text, pos)
		if c != r.literal[0] {
			pos += size
			continue
		}
		end := pos + size
		i := 1
		for i < len(r.literal) && end < len(text) {
			c, size = foldASCII(text, end)
			if c != r.literal[i] {
				break
			}
			end += size
			i++
		}
		if i != len(r.literal) {
			_, size = foldASCII(text, pos)
			pos += size
			continue
		}
		if out.Cap() == 0 {
			out.Grow(len(text))
		}
		out.WriteString(text[last:pos])
		out.WriteString(r.replacement(text[pos:end]))
		pos = end
		last = end
	}
	if last == 0 {
		return text
	}
	out.WriteString(text[last:])
	return out.String()
}

type ArgSuffixPair struct {
	Suffix string
	Val    string
}

type Task struct {
	Src  string
	Dest string
}

func main() {
	force := false
	var args []string
	var fromList []ArgSuffixPair
	var toList []ArgSuffixPair

	fromRe := regexp.MustCompile(`^--from(\d*)$`)
	toRe := regexp.MustCompile(`^--to(\d*)$`)

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "-f" || arg == "--force" {
			force = true
		} else if m := fromRe.FindStringSubmatch(arg); m != nil {
			if i+1 < len(os.Args) {
				fromList = append(fromList, ArgSuffixPair{Suffix: m[1], Val: os.Args[i+1]})
				i++
			} else {
				fmt.Fprintln(os.Stderr, "Error: Missing value for", arg)
				os.Exit(1)
			}
		} else if m := toRe.FindStringSubmatch(arg); m != nil {
			if i+1 < len(os.Args) {
				toList = append(toList, ArgSuffixPair{Suffix: m[1], Val: os.Args[i+1]})
				i++
			} else {
				fmt.Fprintln(os.Stderr, "Error: Missing value for", arg)
				os.Exit(1)
			}
		} else {
			args = append(args, arg)
		}
	}

	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: hyper_copy [options] <source...> <dest>")
		os.Exit(1)
	}

	var replacements []Replacement
	for _, f := range fromList {
		found := false
		for j, t := range toList {
			if t.Suffix == f.Suffix {
				replacements = append(replacements, Replacement{From: f.Val, To: t.Val})
				toList = append(toList[:j], toList[j+1:]...)
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "Error: Missing --to%s for --from%s %s\n", f.Suffix, f.Suffix, f.Val)
			os.Exit(1)
		}
	}

	if len(toList) > 0 {
		fmt.Fprintf(os.Stderr, "Error: Missing --from%s for --to%s %s\n", toList[0].Suffix, toList[0].Suffix, toList[0].Val)
		os.Exit(1)
	}

	replacer := newReplacer(replacements)

	sources := args[:len(args)-1]
	dest := args[len(args)-1]

	var tasks []Task

	destInfo, err := os.Stat(dest)
	if err == nil && destInfo.IsDir() {
		for _, src := range sources {
			baseName := filepath.Base(src)
			newBaseName := replacer.apply(baseName)
			tasks = append(tasks, Task{Src: src, Dest: filepath.Join(dest, newBaseName)})
		}
	} else {
		if len(sources) > 1 {
			fmt.Fprintf(os.Stderr, "Error: target '%s' is not a directory\n", dest)
			os.Exit(1)
		}
		tasks = append(tasks, Task{Src: sources[0], Dest: dest})
	}

	var conflicts []string
	for _, t := range tasks {
		if _, err := os.Stat(t.Dest); err == nil {
			conflicts = append(conflicts, t.Dest)
		}
	}

	if len(conflicts) > 0 && !force {
		fmt.Fprintf(os.Stderr, "Cannot overwrite existing file(s): %s\n", strings.Join(conflicts, ", "))
		fmt.Fprintln(os.Stderr, "Use -f or --force to overwrite.")
		os.Exit(1)
	}

	for _, task := range tasks {
		srcInfo, err := os.Stat(task.Src)
		if err != nil || srcInfo.IsDir() { // Note: original only copies files, no directory copy support
			fmt.Fprintf(os.Stderr, "Error: Source file '%s' does not exist.\n", task.Src)
			os.Exit(1)
		}

		content, err := os.ReadFile(task.Src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file '%s': %v\n", task.Src, err)
			os.Exit(1)
		}

		newContent := replacer.apply(string(content))

		_, err = os.Stat(task.Dest)
		overwritten := err == nil

		err = os.WriteFile(task.Dest, []byte(newContent), srcInfo.Mode())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file '%s': %v\n", task.Dest, err)
			os.Exit(1)
		}

		if overwritten {
			fmt.Printf("%s -> %s (overwrite)\n", task.Src, task.Dest)
		} else {
			fmt.Printf("%s -> %s\n", task.Src, task.Dest)
		}
	}
}
