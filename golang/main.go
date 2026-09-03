package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func rubyCapitalize(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	first := strings.ToUpper(string(runes[0]))
	var rest []string
	for i := 1; i < len(runes); i++ {
		rest = append(rest, strings.ToLower(string(runes[i])))
	}
	return first + strings.Join(rest, "")
}

func preserveCase(match, toStr string) string {
	if toStr == "" {
		return ""
	}

	matchUpper := strings.ToUpper(match)
	matchLower := strings.ToLower(match)
	matchCap := rubyCapitalize(match)

	if match == matchUpper {
		return strings.ToUpper(toStr)
	} else if match == matchLower {
		return strings.ToLower(toStr)
	} else if match == matchCap {
		return rubyCapitalize(toStr)
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

func applyReplacements(text string, replacements []Replacement) string {
	if len(replacements) == 0 {
		return text
	}

	sortedPairs := make([]Replacement, len(replacements))
	copy(sortedPairs, replacements)
	sort.SliceStable(sortedPairs, func(i, j int) bool {
		return len(sortedPairs[i].From) > len(sortedPairs[j].From)
	})

	var patternParts []string
	for _, p := range sortedPairs {
		patternParts = append(patternParts, regexp.QuoteMeta(p.From))
	}
	pattern := "(?i)(" + strings.Join(patternParts, "|") + ")"
	re := regexp.MustCompile(pattern)

	return re.ReplaceAllStringFunc(text, func(match string) string {
		var toStr string
		for _, p := range sortedPairs {
			if strings.EqualFold(match, p.From) {
				toStr = p.To
				break
			}
		}
		return preserveCase(match, toStr)
	})
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

	sources := args[:len(args)-1]
	dest := args[len(args)-1]

	var tasks []Task

	destInfo, err := os.Stat(dest)
	if err == nil && destInfo.IsDir() {
		for _, src := range sources {
			baseName := filepath.Base(src)
			newBaseName := applyReplacements(baseName, replacements)
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

		newContent := applyReplacements(string(content), replacements)

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
