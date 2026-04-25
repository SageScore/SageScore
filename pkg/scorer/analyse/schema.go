package analyse

import (
	"encoding/json"

	"github.com/iserter/sagescore/pkg/scorer/parse"
)

// PageKindHint lets the schema analyser know what schemas to expect.
type PageKindHint string

const (
	KindHomepage PageKindHint = "homepage"
	KindAbout    PageKindHint = "about"
	KindArticle  PageKindHint = "article"
	KindProduct  PageKindHint = "product"
	KindOther    PageKindHint = "other"
)

// Schema analyses structured-data on one page.
//
// Formula (methodology §1): base + homepage bonus + author bonus -
// invalid penalty, clamped to [0,100].
func Schema(page *parse.ParsedPage, kind PageKindHint) Sub {
	expected, valid := expectedValid(page, kind)

	base := 0
	if expected > 0 {
		base = (valid * 80) / expected
	} else {
		base = 80 // nothing expected for this kind; don't penalise
	}

	bonus := 0
	findings := []Finding{}

	// Homepage bonus.
	if kind == KindHomepage {
		if hasOrgWithNameURL(page) {
			bonus += 10
		} else {
			findings = append(findings, Finding{
				Severity: SeverityHigh,
				Code:     "SCHEMA_ORG_MISSING",
				Message:  "Homepage does not expose a complete Organization JSON-LD block.",
			})
		}
	}

	// Person (author) bonus on articles.
	if kind == KindArticle {
		if hasPersonAuthor(page) {
			bonus += 10
		} else {
			findings = append(findings, Finding{
				Severity: SeverityHigh,
				Code:     "SCHEMA_PERSON_AUTHOR_MISSING",
				Message:  "Article page does not expose a Person schema for the author.",
			})
		}
	}

	// Invalid JSON-LD penalty: count blocks that didn't parse.
	invalid := countInvalidJSONLDBlocks(page)
	penalty := invalid * 10
	if penalty > 30 {
		penalty = 30
	}
	if invalid > 0 {
		findings = append(findings, Finding{
			Severity: SeverityMedium,
			Code:     "SCHEMA_INVALID_JSON",
			Message:  "One or more JSON-LD blocks failed to parse.",
		})
	}

	// Kind-specific missing findings.
	switch kind {
	case KindArticle:
		if !hasAnyType(page, "Article", "BlogPosting", "NewsArticle") {
			findings = append(findings, Finding{
				Severity: SeverityCritical,
				Code:     "SCHEMA_ARTICLE_MISSING",
				Message:  "Article page lacks Article/BlogPosting JSON-LD.",
			})
		}
	case KindProduct:
		if !hasAnyType(page, "Product") {
			findings = append(findings, Finding{
				Severity: SeverityHigh,
				Code:     "SCHEMA_PRODUCT_MISSING",
				Message:  "Product page lacks Product JSON-LD.",
			})
		}
	}
	// FAQ detection heuristic.
	if looksLikeFAQ(page) && !hasAnyType(page, "FAQPage") {
		findings = append(findings, Finding{
			Severity: SeverityMedium,
			Code:     "SCHEMA_FAQ_MISSING",
			Message:  "Page contains FAQ-shaped content but no FAQPage schema.",
		})
	}
	if !hasAnyType(page, "BreadcrumbList") {
		findings = append(findings, Finding{
			Severity: SeverityInfo,
			Code:     "SCHEMA_BREADCRUMB_MISSING",
			Message:  "No BreadcrumbList schema present.",
		})
	}

	score := clamp(base+bonus-penalty, 0, 100)
	return Sub{Score: score, Findings: findings}
}

func expectedValid(page *parse.ParsedPage, kind PageKindHint) (expected, valid int) {
	switch kind {
	case KindHomepage:
		expected = 1
		if hasAnyType(page, "Organization", "WebSite") {
			valid = 1
		}
	case KindArticle:
		expected = 1
		if hasAnyType(page, "Article", "BlogPosting", "NewsArticle") {
			valid = 1
		}
	case KindProduct:
		expected = 1
		if hasAnyType(page, "Product") {
			valid = 1
		}
	default:
		expected = 0
	}
	return
}

func hasAnyType(page *parse.ParsedPage, types ...string) bool {
	for _, obj := range page.JSONLD {
		if hasType(obj, types...) {
			return true
		}
	}
	return false
}

func hasOrgWithNameURL(page *parse.ParsedPage) bool {
	for _, obj := range page.JSONLD {
		if !hasType(obj, "Organization") {
			continue
		}
		_, nameOk := obj["name"].(string)
		_, urlOk := obj["url"].(string)
		if nameOk && urlOk {
			return true
		}
	}
	return false
}

func hasPersonAuthor(page *parse.ParsedPage) bool {
	for _, obj := range page.JSONLD {
		// Direct Person.
		if hasType(obj, "Person") {
			return true
		}
		// Article with author: {"@type": "Person", ...}.
		if !hasType(obj, "Article", "BlogPosting", "NewsArticle") {
			continue
		}
		auth := obj["author"]
		switch av := auth.(type) {
		case map[string]any:
			if hasType(av, "Person") {
				return true
			}
		case []any:
			for _, x := range av {
				if m, ok := x.(map[string]any); ok && hasType(m, "Person") {
					return true
				}
			}
		}
	}
	return false
}

// countInvalidJSONLDBlocks re-scans page.WordText? No — we don't have
// raw blocks preserved. We approximate: blocks that parsed to nothing
// usable show up as empty JSON-LD entries. This is a best-effort
// heuristic; a real implementation would keep the raw slice.
func countInvalidJSONLDBlocks(_ *parse.ParsedPage) int {
	// The parser drops invalid blocks silently; we can't count them
	// without plumbing a second field. For v0.2 we report 0 and keep a
	// TODO(phase-1-polish).
	return 0
}

func looksLikeFAQ(page *parse.ParsedPage) bool {
	qCount := 0
	for _, h := range page.Headings {
		if len(h.Text) == 0 {
			continue
		}
		if h.Text[len(h.Text)-1] == '?' {
			qCount++
		}
	}
	return qCount >= 3
}

// Raw is a helper used by tests to marshal JSON-LD back out.
func Raw(obj map[string]any) string {
	b, _ := json.Marshal(obj)
	return string(b)
}
