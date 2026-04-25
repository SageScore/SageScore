package analyse

import (
	"strings"

	"github.com/iserter/sagescore/pkg/scorer/parse"
)

// TechSEO scores the technical-SEO baseline for a page. Domain-level
// sitemap reachability is piped in via sitemapReachable.
func TechSEO(page *parse.ParsedPage, sitemapReachable bool) Sub {
	findings := []Finding{}
	// 8 checks × 12.5 pts = 100.
	const perCheck = 12.5
	score := 0.0

	// Canonical.
	if page.Canonical != "" {
		if urlHostEq(page.Canonical, page.URL) {
			score += perCheck
		}
	} else {
		findings = append(findings, Finding{
			Severity: SeverityMedium,
			Code:     "TECH_CANONICAL_MISSING",
			Message:  "No <link rel=\"canonical\"> in <head>.",
		})
	}

	// Meta description length 50–160.
	md := len(page.MetaDescrip)
	switch {
	case md == 0:
		findings = append(findings, Finding{
			Severity: SeverityMedium,
			Code:     "TECH_META_DESC_MISSING",
			Message:  "No <meta name=\"description\">.",
		})
	case md < 50:
		findings = append(findings, Finding{
			Severity: SeverityLow,
			Code:     "TECH_META_DESC_TOO_SHORT",
			Message:  "Meta description is under 50 characters.",
		})
	case md > 160:
		findings = append(findings, Finding{
			Severity: SeverityLow,
			Code:     "TECH_META_DESC_TOO_LONG",
			Message:  "Meta description exceeds 160 characters.",
		})
	default:
		score += perCheck
	}

	// Title length 30–65.
	tl := len(page.Title)
	switch {
	case tl == 0:
		findings = append(findings, Finding{
			Severity: SeverityHigh,
			Code:     "TECH_TITLE_MISSING",
			Message:  "No <title> in <head>.",
		})
	case tl < 30:
		findings = append(findings, Finding{
			Severity: SeverityLow,
			Code:     "TECH_TITLE_TOO_SHORT",
			Message:  "Title is under 30 characters.",
		})
	case tl > 65:
		findings = append(findings, Finding{
			Severity: SeverityLow,
			Code:     "TECH_TITLE_TOO_LONG",
			Message:  "Title exceeds 65 characters.",
		})
	default:
		score += perCheck
	}

	// OG tags.
	og := page.OGTags
	if og["og:title"] != "" && og["og:description"] != "" && og["og:type"] != "" {
		score += perCheck
	} else {
		findings = append(findings, Finding{
			Severity: SeverityLow,
			Code:     "TECH_OG_MISSING",
			Message:  "OpenGraph tags (og:title, og:description, og:type) are incomplete.",
		})
	}

	// Twitter tags.
	tw := page.TwitterTags
	if tw["twitter:card"] != "" && tw["twitter:title"] != "" {
		score += perCheck
	} else {
		findings = append(findings, Finding{
			Severity: SeverityLow,
			Code:     "TECH_TWITTER_MISSING",
			Message:  "Twitter Card tags are incomplete.",
		})
	}

	// Sitemap reachable.
	if sitemapReachable {
		score += perCheck
	} else {
		findings = append(findings, Finding{
			Severity: SeverityHigh,
			Code:     "TECH_SITEMAP_UNREACHABLE",
			Message:  "/sitemap.xml is not reachable or not a valid sitemap.",
		})
	}

	// HTML size < 300 KB (LCP proxy).
	if page.HTMLSize > 0 && page.HTMLSize < 300*1024 {
		score += perCheck
	} else if page.HTMLSize >= 300*1024 {
		findings = append(findings, Finding{
			Severity: SeverityLow,
			Code:     "TECH_HTML_TOO_LARGE",
			Message:  "HTML response exceeds 300 KB; large payloads hurt LCP and AI-search retrieval.",
		})
	}

	// Render-blocking scripts in <head> ≤ 2.
	if page.RenderBlocking <= 2 {
		score += perCheck
	} else {
		findings = append(findings, Finding{
			Severity: SeverityLow,
			Code:     "TECH_RENDER_BLOCKING_SCRIPTS",
			Message:  "Multiple render-blocking scripts in <head> without async/defer.",
		})
	}

	return Sub{Score: clamp(int(score+0.5), 0, 100), Findings: findings}
}

func urlHostEq(a, b string) bool {
	return strings.EqualFold(hostOf(a), hostOf(b))
}

// hostOf duplicates parse.hostOf to avoid an import cycle with tests
// that stub ParsedPage.
func hostOf(u string) string {
	i := strings.Index(u, "://")
	if i < 0 {
		return ""
	}
	rest := u[i+3:]
	end := len(rest)
	for j, r := range rest {
		if r == '/' || r == '?' || r == '#' {
			end = j
			break
		}
	}
	host := rest[:end]
	if at := strings.LastIndex(host, "@"); at >= 0 {
		host = host[at+1:]
	}
	if colon := strings.Index(host, ":"); colon >= 0 {
		host = host[:colon]
	}
	return strings.ToLower(host)
}
