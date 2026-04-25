package analyse

import (
	"regexp"
	"strings"
	"time"

	"github.com/iserter/sagescore/pkg/scorer/parse"
)

// numericRE matches discrete numeric facts: integers, decimals, percents,
// currency, year references.
var (
	numericRE  = regexp.MustCompile(`(?i)(?:\$|€|£|¥)?\b\d{1,3}(?:,\d{3})+(?:\.\d+)?\b|\b\d+(?:\.\d+)?%?\b|\b(?:19|20)\d{2}\b`)
	attribRE   = regexp.MustCompile(`(?i)\b(according to|said|says|wrote|reports?|notes?|writes?|stated)\b`)
	blockWords = regexp.MustCompile(`\b\w+\b`)
)

// EvidenceInputs bundles what the analyser needs.
type EvidenceInputs struct {
	Page         *parse.ParsedPage
	AuthoritySet map[string]struct{} // authoritative domain set
	LastModHTTP  time.Time           // fallback when JSON-LD absent
	Now          time.Time           // injected for determinism in tests
}

// Evidence scores the Evidence & Citation Readiness dimension. Per page.
//
// Weights (methodology §6): 20+20+20+15+25 = 100.
func Evidence(in EvidenceInputs) Sub {
	findings := []Finding{}
	if in.Now.IsZero() {
		in.Now = time.Now()
	}
	page := in.Page
	if page == nil {
		return Sub{Score: 0}
	}
	score := 0

	// 1. Statistics density (20 pts): ≥3 distinct numeric facts per 1000 words.
	numbers := numericRE.FindAllString(page.WordText, -1)
	uniq := map[string]struct{}{}
	for _, n := range numbers {
		uniq[n] = struct{}{}
	}
	per1000 := 0.0
	if page.WordN > 0 {
		per1000 = float64(len(uniq)) * 1000.0 / float64(page.WordN)
	}
	switch {
	case per1000 >= 3:
		score += 20
	case per1000 >= 1:
		score += 10
	default:
		findings = append(findings, Finding{
			Severity: SeverityHigh,
			Code:     "EVIDENCE_STATISTICS_THIN",
			Message:  "Content carries few concrete numeric facts; Princeton GEO shows statistics raise LLM citation rates ~40%.",
		})
	}

	// 2. Direct quotations with attribution (20 pts).
	q := page.Blockquotes + page.QTags
	hasAttrib := attribRE.MatchString(page.WordText)
	switch {
	case q >= 1 && hasAttrib:
		score += 20
	case q >= 1:
		score += 10
	default:
		findings = append(findings, Finding{
			Severity: SeverityMedium,
			Code:     "EVIDENCE_NO_QUOTATIONS",
			Message:  "No attributed quotations found; quoted-source passages raise LLM citation rates ~41%.",
		})
	}

	// 3. Outbound authoritative citations (20 pts): ≥2 distinct.
	authCount := countAuthoritativeOutbound(page, in.AuthoritySet)
	switch {
	case authCount >= 5:
		score += 20
	case authCount >= 2:
		score += 10 + 2*(authCount-2)
	case authCount == 1:
		score += 5
	default:
		findings = append(findings, Finding{
			Severity: SeverityMedium,
			Code:     "CITATIONS_NO_AUTHORITATIVE_OUTBOUND",
			Message:  "No outbound links to widely recognised authoritative domains.",
		})
	}

	// 4. Internal anchor density (15 pts): 5–15 per 1000 words.
	internal := 0
	for _, a := range page.Anchors {
		if a.Internal {
			internal++
		}
	}
	per1kInternal := 0.0
	if page.WordN > 0 {
		per1kInternal = float64(internal) * 1000.0 / float64(page.WordN)
	}
	switch {
	case per1kInternal >= 5 && per1kInternal <= 15:
		score += 15
	case per1kInternal > 0 && per1kInternal < 5:
		score += int(15 * (per1kInternal / 5))
		findings = append(findings, Finding{
			Severity: SeverityLow,
			Code:     "CITATIONS_THIN_INTERNAL_LINKING",
			Message:  "Internal linking is thin; 5–15 links per 1000 words is the recommended band.",
		})
	case per1kInternal > 15:
		score += 8
		findings = append(findings, Finding{
			Severity: SeverityLow,
			Code:     "CITATIONS_DENSE_INTERNAL_LINKING",
			Message:  "Internal links are very dense; may dilute signal.",
		})
	}

	// 5. Freshness (25 pts).
	mod := page.DateModified
	if mod.IsZero() {
		mod = page.DatePublished
	}
	if mod.IsZero() {
		mod = in.LastModHTTP
	}
	var age time.Duration
	if !mod.IsZero() {
		age = in.Now.Sub(mod)
	}
	days := age.Hours() / 24
	switch {
	case !mod.IsZero() && days <= 180:
		score += 25
	case !mod.IsZero() && days <= 365:
		score += 18
	case !mod.IsZero() && days <= 730:
		score += 10
		findings = append(findings, Finding{
			Severity: SeverityLow,
			Code:     "CITATIONS_STALE_CONTENT",
			Message:  "Content has not been updated in over a year; Perplexity drops citation rates sharply past 180 days.",
		})
	default:
		findings = append(findings, Finding{
			Severity: SeverityHigh,
			Code:     "CITATIONS_STALE_CONTENT",
			Message:  "No recent dateModified signal; freshness is a major AEO factor.",
		})
	}

	return Sub{Score: clamp(score, 0, 100), Findings: findings}
}

func countAuthoritativeOutbound(page *parse.ParsedPage, set map[string]struct{}) int {
	seen := map[string]struct{}{}
	for _, a := range page.Anchors {
		if a.Internal {
			continue
		}
		host := anchorHost(a.Href)
		if host == "" {
			continue
		}
		if _, ok := set[host]; !ok {
			continue
		}
		if _, dup := seen[host]; dup {
			continue
		}
		seen[host] = struct{}{}
	}
	return len(seen)
}

func anchorHost(href string) string {
	i := strings.Index(href, "://")
	if i < 0 {
		return ""
	}
	rest := href[i+3:]
	end := len(rest)
	for j, r := range rest {
		if r == '/' || r == '?' || r == '#' {
			end = j
			break
		}
	}
	host := rest[:end]
	if colon := strings.Index(host, ":"); colon >= 0 {
		host = host[:colon]
	}
	return strings.ToLower(host)
}
