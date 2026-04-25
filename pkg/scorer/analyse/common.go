// Package analyse holds the six dimension analysers. Each analyser is
// a pure function over a ParsedPage (or a RobotsSummary for the
// domain-level ones) and returns a Sub-score plus ordered findings.
//
// Methodology: docs/methodology.md is the normative spec. If this
// package disagrees with that doc, the doc is right.
package analyse

import "github.com/iserter/sagescore/pkg/scorer/parse"

// Finding is a single observation produced by an analyser.
type Finding struct {
	Severity Severity
	Code     string
	Message  string
	Evidence string
	FixCTA   string
}

// Severity orders findings on the audit page.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Sub is one dimension's score and its findings.
type Sub struct {
	Score    int
	Findings []Finding
}

// clamp returns x bounded to [lo, hi].
func clamp(x, lo, hi int) int {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

// hasType reports whether a JSON-LD object declares any of the types.
// Works for both string and []string @type forms.
func hasType(obj map[string]any, types ...string) bool {
	t, ok := obj["@type"]
	if !ok {
		return false
	}
	match := func(s string) bool {
		for _, want := range types {
			if s == want {
				return true
			}
		}
		return false
	}
	switch tv := t.(type) {
	case string:
		return match(tv)
	case []any:
		for _, x := range tv {
			if s, ok := x.(string); ok && match(s) {
				return true
			}
		}
	}
	return false
}

// findAny returns the first JSON-LD object across all pages matching
// any of the types.
func findAny(pages []*parse.ParsedPage, types ...string) map[string]any {
	for _, p := range pages {
		if p == nil {
			continue
		}
		for _, obj := range p.JSONLD {
			if hasType(obj, types...) {
				return obj
			}
		}
	}
	return nil
}
