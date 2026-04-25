package analyse

import (
	"strings"
	"testing"
	"time"

	"github.com/iserter/sagescore/pkg/scorer/parse"
)

func TestEvidence_StaleContent(t *testing.T) {
	now := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	old := now.Add(-3 * 365 * 24 * time.Hour)
	page := &parse.ParsedPage{
		WordText:     "hello",
		Words:        []string{"hello"},
		WordN:        1,
		DateModified: old,
	}
	sub := Evidence(EvidenceInputs{Page: page, Now: now, AuthoritySet: map[string]struct{}{}})
	seen := false
	for _, f := range sub.Findings {
		if f.Code == "CITATIONS_STALE_CONTENT" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("CITATIONS_STALE_CONTENT not emitted for 3-year-old dateModified")
	}
}

func TestEvidence_FreshFullPoints(t *testing.T) {
	now := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	page := &parse.ParsedPage{
		WordText:     "In 2025, 47% of readers cited 3 sources according to the survey. According to Dr. Smith the trend is up.",
		DateModified: now.Add(-30 * 24 * time.Hour),
		Blockquotes:  1,
	}
	page.Words = strings.Fields(page.WordText)
	page.WordN = len(page.Words)
	sub := Evidence(EvidenceInputs{Page: page, Now: now, AuthoritySet: map[string]struct{}{}})
	if sub.Score < 40 {
		t.Fatalf("expected healthy score for fresh quoted+numeric content, got %d", sub.Score)
	}
}

func TestEvidence_AuthoritativeOutbound(t *testing.T) {
	now := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	authSet := map[string]struct{}{
		"wikipedia.org": {},
		"nih.gov":       {},
	}
	page := &parse.ParsedPage{
		WordText:     "Some content.",
		WordN:        2,
		DateModified: now.Add(-30 * 24 * time.Hour),
		Anchors: []parse.Anchor{
			{Href: "https://wikipedia.org/x", Internal: false},
			{Href: "https://nih.gov/y", Internal: false},
			{Href: "https://unknown.com/z", Internal: false},
		},
	}
	sub := Evidence(EvidenceInputs{Page: page, Now: now, AuthoritySet: authSet})
	for _, f := range sub.Findings {
		if f.Code == "CITATIONS_NO_AUTHORITATIVE_OUTBOUND" {
			t.Fatal("unexpected CITATIONS_NO_AUTHORITATIVE_OUTBOUND with 2 authoritative outbounds")
		}
	}
}
