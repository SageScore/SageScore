// Command lint-content scans templates and content fragments for
// evaluative language that the SageScore brief explicitly forbids
// (PRD §6 "Defensive language", Technical Plan §6.4).
//
// Phase 0: scaffold. The full implementation walks templates/ and
// content/ in Phase 3. Until then it exits zero on an empty repo.
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// blockedWords are forbidden in any user-facing template or content
// fragment about an audited site. Audit pages must describe HTML
// signals, never grade the business.
var blockedWords = []string{
	"bad", "broken", "neglected", "lazy",
	"incompetent", "outdated", "garbage", "terrible",
}

var blockedRE = regexp.MustCompile(`(?i)\b(` + strings.Join(blockedWords, "|") + `)\b`)

// scanDirs are the directories that contain user-facing copy. Phase 3
// adds templates/ and content/.
var scanDirs = []string{"templates", "content"}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	violations := 0
	for _, dir := range scanDirs {
		path := filepath.Join(root, dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// Phase 0: directories don't exist yet. Skip silently.
			continue
		}
		err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(p))
			if ext != ".gohtml" && ext != ".html" && ext != ".md" {
				return nil
			}
			body, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(body), "\n") {
				if m := blockedRE.FindString(line); m != "" {
					fmt.Fprintf(os.Stderr,
						"%s:%d: blocked word %q (defensive-language policy)\n",
						p, i+1, m)
					violations++
				}
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "lint-content: %v\n", err)
			os.Exit(2)
		}
	}

	if violations > 0 {
		fmt.Fprintf(os.Stderr, "\n%d violation(s) found.\n", violations)
		os.Exit(1)
	}
	fmt.Println("lint-content: ok")
}
