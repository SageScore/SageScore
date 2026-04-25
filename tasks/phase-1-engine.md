# Phase 1 — Scoring Engine

**Window:** 2026-05-19 → 2026-05-25 (5 working days)
**Goal:** A Go scorer that takes a domain and returns a complete `Audit` struct with all 6 sub-scores and ≥3 article PageAudits. Usable as a CLI today, importable as a library tomorrow.
**Exit gate:** `sagescore audit example.com -o audit.json` produces a valid `Audit` JSON for 5 hand-picked test domains; golden tests pass; OSS-quality README in `pkg/scorer/`.

---

## Why this phase exists

The scorer is the load-bearing wall. If it's wrong, every audit page is wrong, the OSS defence is hollow, and the cold-outreach payload becomes a liability. Build it standalone before any web layer touches it.

---

## Sequencing

The 6 analysers are mostly independent, but order matters for incremental verification:

```
fetch+robots → crawler+sampling → schema → ai_crawlers → tech_seo → content → entity → citations → aggregate
                                  ◄────── unblocks CLI demo ────────►
```

After `tech_seo` the CLI can produce useful (if partial) output. That's the natural mid-phase checkpoint.

---

## Day-by-day plan

### Day 1 — Fetch, robots, project plumbing

- [ ] `pkg/scorer/fetch/fetch.go`:
  - `Fetch(ctx, url) (Response, error)` with the SageScoreBot UA, 30s timeout, 5MB body cap, redirect follow with same-host preference, retry policy from Technical Plan §3.4.
  - Returns `Response{StatusCode, Header, Body []byte, FinalURL, FetchedAt}`.
- [ ] `pkg/scorer/parse/robots.go`:
  - Parse `/robots.txt`. Use `github.com/temoto/robotstxt` (battle-tested, used by colly).
  - `RobotsSummary{Allowed map[string]bool, HasLLMSTxt bool, HasSageScoreBotRule bool, RawBytes []byte}`.
  - Helper: `IsAllowed(ua, path) bool`, used by both opt-out logic and analysers.
- [ ] `pkg/scorer/parse/sitemap.go`:
  - Parse `/sitemap.xml` (and sitemap-index files, recursing one level).
  - `SitemapSummary{URLs []SitemapURL, IsIndex bool, FetchedAt time.Time}` where each URL has `Loc`, `LastMod`, `Priority`.
  - Cap at 1000 URLs read.
- [ ] Unit tests with HTML/XML fixtures under `pkg/scorer/testdata/`.

**End of day:** `go test ./pkg/scorer/...` green. CLI does nothing useful yet.

### Day 2 — Crawler and URL sampling

- [ ] `pkg/scorer/crawl/crawler.go`:
  - Wrap `gocolly/colly v2`. Configure UA, async, `LimitRule{Parallelism: 1, Delay: 5s}`, robots-aware, max 5MB response.
  - `Crawl(ctx, domain, plan SamplePlan) ([]Response, error)`.
- [ ] `pkg/scorer/crawl/sampler.go`:
  - Implement URL sampling per Technical Plan §3.4 deterministically.
  - Article discovery pipeline: sitemap path patterns (`/(blog|articles|posts|news|insights|guides|resources|knowledge)/[^/]+`) → lastmod-tagged URLs at depth ≥ 2 → homepage anchors → blog index outlinks.
  - Stops once 3 article candidates are found; returns the full plan (always-fetch + articles + diversity fillers, ≤10).
  - Pure function over a `SitemapSummary` + homepage HTML — easy to unit-test.
- [ ] `pkg/scorer/parse/page.go`:
  - Parse a fetched HTML page once into a reusable `ParsedPage` struct (title, meta, headings, JSON-LD blocks, anchors, body text, og/twitter tags, canonical, dateModified, has `<article>`, word count).
  - All analysers consume `ParsedPage`, never the raw bytes — single parse pass.
- [ ] Page classification helper: `Classify(page ParsedPage, sourceURL string, foundVia ArticleSource) PageKind`. Article requires either `Article`/`BlogPosting` JSON-LD OR `<article>` element with ≥300 words.

**End of day:** `Crawl()` returns 5–10 `Response` objects for a real test domain on `make audit DOMAIN=example.com`.

### Day 3 — Schema + AI-crawler analysers

- [ ] `pkg/scorer/analyse/schema.go`:
  - Iterate JSON-LD blocks per page, validate against expected types for that page kind.
  - Findings: `SCHEMA_ARTICLE_MISSING`, `SCHEMA_FAQ_MISSING`, `SCHEMA_ORG_MISSING`, `SCHEMA_INVALID_JSON`, `SCHEMA_MISSING_REQUIRED_PROP`.
  - Per-page score = % of expected schemas present and valid.
- [ ] `pkg/scorer/analyse/ai_crawlers.go`:
  - Domain-level. Inputs: `RobotsSummary`.
  - Score components: explicit allow for each of GPTBot/PerplexityBot/ClaudeBot/Google-Extended/Applebot-Extended (10 pts each), `llms.txt` present (25 pts), wildcard-disallow penalty (-25 pts).
  - Findings: `AI_CRAWLER_BLOCKED_<UA>`, `LLMS_TXT_MISSING`, `WILDCARD_DISALLOW`.
- [ ] Unit tests with at least 5 fixtures per analyser, each covering one finding code.

### Day 4 — Tech SEO + Content analysers

- [ ] `pkg/scorer/analyse/tech_seo.go`:
  - Per-page. Canonical present + matches URL, meta description 50–160 chars, title 30–65 chars, sitemap reachable (domain-level signal piped in), OG + Twitter cards.
  - Findings: `TECH_CANONICAL_MISSING`, `TECH_META_DESC_TOO_SHORT`, `TECH_TITLE_TOO_LONG`, `TECH_OG_MISSING`, `TECH_SITEMAP_UNREACHABLE`.
- [ ] `pkg/scorer/analyse/content.go`:
  - Per-page. Heading hierarchy validity (no h3 without an h2 above it), mean paragraph length, BLUF heuristic (first sentence < 200 chars, contains a noun + verb via simple POS heuristics — no NLP libraries), Flesch reading ease.
  - Findings: `CONTENT_H_HIERARCHY_BROKEN`, `CONTENT_BLUF_WEAK`, `CONTENT_PARAGRAPHS_TOO_LONG`, `CONTENT_READING_EASE_LOW`.
- [ ] CLI checkpoint: `sagescore audit example.com` now prints a partial score with 3 of 6 dimensions filled.

### Day 5 — Entity + Citations + aggregation + CLI polish + golden tests

- [ ] `pkg/scorer/analyse/entity.go`:
  - Domain-level. Inputs: homepage `ParsedPage`, `/about` `ParsedPage`, footer text.
  - Author/Org schema, About page presence, NAP regex (phone/email/address), `sameAs` social links, `<meta name="author">`.
  - Findings: `ENTITY_ABOUT_MISSING`, `ENTITY_NAP_INCOMPLETE`, `ENTITY_SOCIAL_MISSING`, `ENTITY_ORG_SCHEMA_MISSING`.
- [ ] `pkg/scorer/analyse/citations.go`:
  - Per-page. Outbound link analysis using `pkg/scorer/data/authoritative.txt`, internal anchor density, `dateModified` or last-mod header recency.
  - Findings: `CITATIONS_NO_AUTHORITATIVE_OUTBOUND`, `CITATIONS_THIN_INTERNAL_LINKING`, `CITATIONS_STALE_CONTENT`.
- [ ] `pkg/scorer/score.go`:
  - Per-page roll-up: weighted across schema (25%) / content (20%) / tech_seo (10%) / citations (10%) — these four are page-level. Renormalise to 0-100.
  - Domain-level dimensions: ai_crawlers (20%) and entity_clarity (15%) computed once.
  - Domain score: weighted sum of (page-level dimensions averaged with homepage 2× / articles 1× / others 1×) + domain-level dimensions.
  - `ARTICLES_INSUFFICIENT_SAMPLE` finding when fewer than 3 articles found; cap citation sub-score at 70.
  - Banker's rounding to int.
- [ ] `cmd/sagescore/main.go`:
  - Cobra-style CLI: `sagescore audit <domain> [-o file.json] [-v]`.
  - Pretty-print mode: score + top 3 findings per dimension.
  - JSON mode: full `Audit` struct.
- [ ] **Golden tests** (`pkg/scorer/scorer_golden_test.go`):
  - 5 hand-picked real domains (cover WordPress blog, Shopify store, Webflow marketing site, Next.js docs, plain HTML).
  - Use `dnaeon/go-vcr` to record HTTP responses on first run, replay on CI.
  - Assert exact score, exact set of finding codes per dimension, exact page count, exact PageKind per page.
  - Re-record only when scorer version bumps; treat as a manual gate.
- [ ] **Property tests** on aggregation: monotonicity (raising any sub-score never lowers total), bounds (total ∈ [0, 100]), sub-score bounds.
- [ ] `pkg/scorer/README.md`: how to use the library, methodology link, weights table, version policy, "no LLM calls" promise, Apache-2.0 license badge.

---

## Acceptance criteria

- [ ] All 6 analysers implemented with ≥5 fixture tests each.
- [ ] CLI runs end-to-end on 5 test domains.
- [ ] Golden tests pass deterministically on CI (no network).
- [ ] Property tests pass.
- [ ] `pkg/scorer/README.md` is OSS-publishable.
- [ ] `go test ./... -race -count=10` is green (no flakes from concurrency).

---

## Risks specific to this phase

| Risk | Mitigation |
|---|---|
| BLUF heuristic produces false positives | Keep the rule simple (length + has verb). The audit page should show evidence, not lecture. False positives are caught at golden-test review. |
| `colly` rate-limit interacts oddly with redirects | Test against a fixture site that redirects across hosts; assert host-bucketed limiting works. |
| Schema validation libraries pull in heavy deps | Don't validate against a full schema.org JSON Schema. Just check expected `@type` values exist and required props for the types we care about are non-empty. |
| Article discovery yields <3 on small marketing sites | Expected; emit `ARTICLES_INSUFFICIENT_SAMPLE` and proceed. Don't try to be clever with anchor crawling. |
| One real domain in golden tests changes its HTML | That's why we record with VCR. The recording is the contract. Re-record only with explicit version bump. |

---

## Deliverables checklist

- `pkg/scorer/scorer.go` — public `Audit(domain) (Result, error)`.
- `pkg/scorer/fetch/`, `pkg/scorer/crawl/`, `pkg/scorer/parse/`, `pkg/scorer/analyse/` (6 files), `pkg/scorer/score.go`.
- `cmd/sagescore/main.go` — usable CLI.
- Golden test corpus under `pkg/scorer/testdata/golden/` with VCR cassettes.
- `pkg/scorer/README.md`.

---

## Estimated effort

- Day 1: 6h (fetch + robots + sitemap, plus tests)
- Day 2: 6h (crawler + sampler, deterministic article discovery is the tricky bit)
- Day 3: 6h (schema + ai_crawlers)
- Day 4: 6h (tech_seo + content + BLUF heuristic)
- Day 5: 8h (entity + citations + aggregation + CLI + goldens — longest day)

**Total: ~32h, tight but doable in 5 days.**
