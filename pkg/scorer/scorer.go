// Package scorer is the SageScore audit engine.
//
// Phase 0 status: skeleton. The Audit function is stubbed to return
// ErrNotImplemented; the full implementation lands in Phase 1.
//
// Methodology is documented in docs/methodology.md. Weights, sub-score
// definitions, and the page-sampling rules are normative there; this
// package is the executable specification of those rules.
package scorer

import (
	"context"
	"errors"
	"time"
)

// Version is the scorer's SemVer string. It is stamped onto every audit
// result and printed at the top of every audit page; bump it when the
// scoring algorithm changes in any user-observable way (see methodology
// versioning policy).
const Version = "0.2.0-dev"

// ErrNotImplemented is returned by stubbed methods during the Phase 0
// scaffold. Phase 1 replaces these with real implementations.
var ErrNotImplemented = errors.New("scorer: not implemented (Phase 1)")

// Dimension is one of the six scoring dimensions defined in
// docs/methodology.md.
type Dimension string

const (
	DimStructuredData   Dimension = "structured_data"
	DimAICrawlerAccess  Dimension = "ai_crawler_access"
	DimContentStructure Dimension = "content_structure"
	DimEntityClarity    Dimension = "entity_clarity"
	DimTechSEOBaseline  Dimension = "tech_seo_baseline"
	DimEvidenceCitation Dimension = "evidence_citation"
)

// Weights encodes the locked v0.2.0 weights. Sum is 100. Changing any
// value here is a major scorer-version bump. Rationale for the shift
// from v0.1.0 is in docs/methodology.md and docs/decisions.md D-11.
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
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Finding is a single observation produced by an analyser.
type Finding struct {
	Severity Severity
	Code     string // e.g. "SCHEMA_ARTICLE_MISSING"
	Message  string
	Evidence string // free-form, may include a snippet
	FixCTA   string // optional upsell link to a SAGE GRIDS plugin
}

// Sub is one dimension's score and its findings.
type Sub struct {
	Score    int       // 0-100
	Findings []Finding // ordered by descending Severity
}

// PageAudit is the per-page result, persisted alongside the parent Audit.
// Only the page-level dimensions populate Subscores here.
type PageAudit struct {
	URL        string
	Kind       PageKind
	Score      int
	Subscores  map[Dimension]Sub
	WordCount  int
	StatusCode int
	FetchedAt  time.Time
}

// RobotsSummary captures domain-level robots.txt facts.
type RobotsSummary struct {
	Allowed             map[string]bool // user-agent → allowed
	HasLLMSTxt          bool
	HasLLMSFullTxt      bool
	HasSageScoreBotRule bool
}

// SitemapSummary captures domain-level sitemap facts.
type SitemapSummary struct {
	Reachable bool
	URLs      []SitemapURL
	IsIndex   bool
	FetchedAt time.Time
}

// SitemapURL is a single sitemap entry.
type SitemapURL struct {
	Loc      string
	LastMod  time.Time
	Priority float64
}

// Audit is the full result for a domain.
type Audit struct {
	Domain        string
	FetchedAt     time.Time
	ScorerVersion string
	Score         int
	Subscores     map[Dimension]Sub
	Pages         []PageAudit
	Robots        RobotsSummary
	Sitemap       SitemapSummary
	CMS           string
	Errors        []string
}

// Run executes a full audit for the given domain.
//
// Phase 0: stub. Phase 1: real implementation crawling up to 10 pages
// (≥3 articles where possible) and computing all six sub-scores.
func Run(ctx context.Context, domain string) (*Audit, error) {
	_ = ctx
	_ = domain
	return nil, ErrNotImplemented
}
