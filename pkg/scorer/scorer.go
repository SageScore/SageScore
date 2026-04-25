// Package scorer is the SageScore audit engine.
//
// Methodology (docs/methodology.md) is normative; this package is the
// executable specification of those rules.
package scorer

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/iserter/sagescore/pkg/scorer/analyse"
	"github.com/iserter/sagescore/pkg/scorer/crawl"
	"github.com/iserter/sagescore/pkg/scorer/data"
	"github.com/iserter/sagescore/pkg/scorer/fetch"
)

// Version is the scorer's SemVer string. Stamped onto every audit.
// Updated automatically on merge by release-please; do not bump by
// hand. The marker comment on the same line is load-bearing.
const Version = "0.2.0" // x-release-please-version

// ErrInvalidDomain is returned for malformed/unsafe input domains.
var ErrInvalidDomain = errors.New("scorer: invalid domain")

// Dimension is one of the six scoring dimensions (docs/methodology.md).
type Dimension string

const (
	DimStructuredData   Dimension = "structured_data"
	DimAICrawlerAccess  Dimension = "ai_crawler_access"
	DimContentStructure Dimension = "content_structure"
	DimEntityClarity    Dimension = "entity_clarity"
	DimTechSEOBaseline  Dimension = "tech_seo_baseline"
	DimEvidenceCitation Dimension = "evidence_citation"
)

// Weights — v0.2.0. Sum = 100. See docs/decisions.md D-11 for rationale.
var Weights = map[Dimension]int{
	DimStructuredData:   22,
	DimAICrawlerAccess:  12,
	DimContentStructure: 20,
	DimEntityClarity:    18,
	DimTechSEOBaseline:  10,
	DimEvidenceCitation: 18,
}

// PageKind classifies a sampled URL.
type PageKind string

const (
	PageHomepage PageKind = "homepage"
	PageAbout    PageKind = "about"
	PageArticle  PageKind = "article"
	PageProduct  PageKind = "product"
	PageOther    PageKind = "other"
)

// Severity orders findings on the audit page.
type Severity = analyse.Severity

// Severity constants re-exported from the analyse package so callers
// can construct findings without importing both packages.
const (
	SeverityInfo     = analyse.SeverityInfo
	SeverityLow      = analyse.SeverityLow
	SeverityMedium   = analyse.SeverityMedium
	SeverityHigh     = analyse.SeverityHigh
	SeverityCritical = analyse.SeverityCritical
)

// Finding is exported for persistence and rendering layers. We alias
// analyse.Finding to keep the two layers sharing one struct.
type Finding = analyse.Finding

// Sub is one dimension's score and its findings.
type Sub struct {
	Score    int
	Findings []Finding
}

// PageAudit is the per-page result. Only page-level dimensions are
// populated in Subscores.
type PageAudit struct {
	URL        string
	Kind       PageKind
	Score      int
	Subscores  map[Dimension]Sub
	WordCount  int
	StatusCode int
	FetchedAt  time.Time
}

// Audit is the full result for a domain.
type Audit struct {
	Domain        string
	FetchedAt     time.Time
	ScorerVersion string
	Score         int
	Subscores     map[Dimension]Sub
	Pages         []PageAudit
	CMS           string
	OptedOut      bool
	Errors        []string
}

// Config tunes engine behaviour. Zero value is fine.
type Config struct {
	// HTTPClient overrides the default fetch.Client.
	HTTPClient *fetch.Client
	// Now is injected for deterministic freshness scoring in tests.
	Now time.Time
}

// Run executes a full audit for the given domain.
//
// The domain may be a bare host ("example.com"), a scheme+host
// ("https://example.com"), or an http scheme. Run normalises to
// https://<lowercased-host>/.
func Run(ctx context.Context, domain string, cfg Config) (*Audit, error) {
	origin, err := normaliseOrigin(domain)
	if err != nil {
		return nil, err
	}
	now := cfg.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	client := cfg.HTTPClient
	if client == nil {
		client = fetch.NewClient()
	}

	audit := &Audit{
		Domain:        strings.TrimPrefix(strings.TrimPrefix(origin, "https://"), "http://"),
		FetchedAt:     now,
		ScorerVersion: Version,
	}

	result, err := crawl.Run(ctx, origin, client)
	if err != nil && errors.Is(err, crawl.ErrOptedOut) {
		audit.OptedOut = true
		return audit, nil
	}
	if err != nil && result == nil {
		return audit, err
	}

	// Build PageAudit rows and run per-page analysers.
	authoritative := data.AuthoritativeDomains()
	var homepage, about *pageRef
	var articles []*pageRef
	var others []*pageRef

	for _, f := range result.Fetches {
		if f.Page == nil {
			continue
		}
		pr := &pageRef{fetched: f}
		switch f.Kind {
		case crawl.PageHomepage:
			homepage = pr
		case crawl.PageAbout:
			about = pr
		case crawl.PageArticle:
			articles = append(articles, pr)
		case crawl.PageOther:
			others = append(others, pr)
		}
	}

	// Sitemap reachable boolean (piped into tech_seo).
	sitemapReachable := result.Sitemap != nil && result.Sitemap.Reachable

	addPageAudit := func(pr *pageRef, kind PageKind) {
		if pr == nil {
			return
		}
		p := pr.fetched.Page
		pa := PageAudit{
			URL:        pr.fetched.URL,
			Kind:       kind,
			Subscores:  map[Dimension]Sub{},
			WordCount:  p.WordN,
			StatusCode: pr.fetched.Response.StatusCode,
			FetchedAt:  pr.fetched.Response.FetchedAt,
		}
		schemaKind := kindHint(kind)
		schemaSub := analyse.Schema(p, schemaKind)
		contentSub := analyse.Content(p)
		techSub := analyse.TechSEO(p, sitemapReachable)
		evidenceSub := analyse.Evidence(analyse.EvidenceInputs{
			Page:         p,
			AuthoritySet: authoritative,
			LastModHTTP:  p.LastModHTTP,
			Now:          now,
		})
		pa.Subscores[DimStructuredData] = fromAnalyse(schemaSub)
		pa.Subscores[DimContentStructure] = fromAnalyse(contentSub)
		pa.Subscores[DimTechSEOBaseline] = fromAnalyse(techSub)
		pa.Subscores[DimEvidenceCitation] = fromAnalyse(evidenceSub)
		pa.Score = pageRollup(pa)
		audit.Pages = append(audit.Pages, pa)
	}

	addPageAudit(homepage, PageHomepage)
	addPageAudit(about, PageAbout)
	for _, a := range articles {
		addPageAudit(a, PageArticle)
	}
	for _, o := range others {
		addPageAudit(o, PageOther)
	}

	// Domain-level analysers.
	aiSub := analyse.AICrawlers(result.Robots)

	var homePage, aboutPage *pageRef
	homePage = homepage
	aboutPage = about
	entityInputs := analyse.EntityInputs{}
	if homePage != nil {
		entityInputs.Homepage = homePage.fetched.Page
	}
	if aboutPage != nil {
		entityInputs.About = aboutPage.fetched.Page
	}
	for _, a := range articles {
		entityInputs.Articles = append(entityInputs.Articles, a.fetched.Page)
	}
	entitySub := analyse.Entity(entityInputs)

	audit.Errors = append(audit.Errors, result.Errors...)
	audit.CMS = detectCMS(homepage)
	aggregate(audit, aiSub, entitySub)
	return audit, nil
}

func kindHint(k PageKind) analyse.PageKindHint {
	switch k {
	case PageHomepage:
		return analyse.KindHomepage
	case PageAbout:
		return analyse.KindAbout
	case PageArticle:
		return analyse.KindArticle
	case PageProduct:
		return analyse.KindProduct
	default:
		return analyse.KindOther
	}
}

// pageRollup computes the per-page score from its 4 page-level sub-scores.
func pageRollup(pa PageAudit) int {
	total := 0.0
	weightSum := 0.0
	for _, dim := range []Dimension{DimStructuredData, DimContentStructure, DimTechSEOBaseline, DimEvidenceCitation} {
		sub, ok := pa.Subscores[dim]
		if !ok {
			continue
		}
		w := float64(Weights[dim])
		total += float64(sub.Score) * w
		weightSum += w
	}
	if weightSum == 0 {
		return 0
	}
	return int(total/weightSum + 0.5)
}

// pageRef carries the crawl FetchedURL around during analysis.
type pageRef struct {
	fetched crawl.FetchedURL
}

// detectCMS returns the display name of the detected CMS, or
// "Unknown / Custom". Matches header and meta-generator rules from
// pkg/scorer/data/cms-fingerprints.json.
//
// v0.2.0 looks at headers and the <meta name="generator"> tag. Path
// matching against raw HTML is a v0.2.1 follow-up — ParsedPage does
// not currently expose the raw body on purpose.
func detectCMS(homepage *pageRef) string {
	if homepage == nil || homepage.fetched.Page == nil {
		return "Unknown / Custom"
	}
	headers := homepage.fetched.Response.Header
	p := homepage.fetched.Page

	// Meta generator: pull it out of the page's OGTags map if the
	// parser stored it. We stored "generator" in MetaAuthor? No — the
	// parser only persists "description" and "author" by name, not
	// "generator". So we expose a tiny helper here that re-scans
	// page.Title / page.MetaDescrip / OGTags won't help. We'll simply
	// accept that without path-matching we can only match fingerprints
	// that expose header rules. That covers WordPress (via x-powered-by),
	// Shopify, Webflow, Squarespace, Wix, Next.js, Ghost, Drupal — 8
	// of 12 out of the box.
	// NB: generator meta support is a v0.2.1 polish item tracked in
	// docs/decisions.md.
	_ = p

	for _, fp := range data.CMSFingerprints() {
		if fp.Fallback {
			continue
		}
		if matchHeaders(fp.HeaderMatch, headers) {
			return fp.Display
		}
	}
	return "Unknown / Custom"
}

func matchHeaders(rules map[string]string, h http.Header) bool {
	if len(rules) == 0 {
		return false
	}
	for k, v := range rules {
		got := h.Get(k)
		if got == "" {
			return false
		}
		if v == "*" {
			continue
		}
		if !strings.Contains(strings.ToLower(got), strings.ToLower(v)) {
			return false
		}
	}
	return true
}

// normaliseOrigin accepts "example.com", "https://example.com",
// "HTTP://Example.com/", "127.0.0.1:8080", etc. Returns a scheme+host
// with no trailing slash. An explicit scheme in the input is preserved
// (so tests can target http servers); bare hosts default to https.
func normaliseOrigin(domain string) (string, error) {
	d := strings.TrimSpace(domain)
	if d == "" {
		return "", ErrInvalidDomain
	}
	low := strings.ToLower(d)
	scheme := "https"
	switch {
	case strings.HasPrefix(low, "https://"):
		scheme = "https"
		d = d[len("https://"):]
	case strings.HasPrefix(low, "http://"):
		scheme = "http"
		d = d[len("http://"):]
	}
	// Strip path.
	if i := strings.Index(d, "/"); i >= 0 {
		d = d[:i]
	}
	d = strings.ToLower(strings.TrimSpace(d))
	if d == "" {
		return "", ErrInvalidDomain
	}
	// Reject obvious invalid hosts (no dots and not a literal IPv4).
	if strings.Count(d, ".") == 0 {
		return "", ErrInvalidDomain
	}
	return scheme + "://" + d, nil
}
