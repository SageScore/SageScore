# SageScore Methodology

**Version:** 0.2.0
**Last updated:** 2026-04-25
**Status:** Locked for v0.1 launch.
**Prior version:** 0.1.0 (initial draft, superseded before any public audit was produced).

This document is the **canonical scoring specification**. The audit pages on `sagescore.org` link directly to it. Any change to weights, sub-score definitions, or thresholds requires a scorer version bump and is tracked in this file's git history.

The v0.2.0 revision incorporates empirical findings from the Princeton GEO paper (Aggarwal et al., KDD 2024), the Structural Feature Engineering GEO study, and 2025–2026 citation-pattern benchmarks comparing ChatGPT, Perplexity, Claude, and Google AI Overviews. See `docs/decisions.md` D-11 for the change log.

---

## What the score is

**SageScore** is a number from 0 to 100 that estimates how prepared a website is to be cited by AI-search systems (ChatGPT, Perplexity, Claude, Google AI Overviews, etc.). It is computed entirely from public on-page signals — HTML, robots.txt, sitemap, llms.txt — fetched over plain HTTP. **No LLM API calls are made.** The score is reproducible: same domain + same scorer version = same score.

## What the score is not

- **Not a verdict on the business.** The score describes the page's HTML signals, not the quality of the company.
- **Not a Google ranking factor.** Google does not use SageScore. Improving SageScore may incidentally improve traditional SEO, but that is not the goal.
- **Not a real-time citation tracker.** We do not query LLMs to see if the site is cited. That category is crowded; we are deliberately staying out of it.
- **Not an editorial judgement.** No human grades any site. The score is a deterministic function of public HTML.
- **Not platform-specific.** Different AI-search platforms weight different signals (ChatGPT favours encyclopaedic content; Perplexity rewards freshness; Google AIO relies on schema). SageScore approximates the *union* of these, not any single platform.

---

## The six dimensions (v0.2.0 weights)

| # | Dimension | Weight | Level | What it measures |
|---|---|---|---|---|
| 1 | Structured Data | 22% | Page + Domain | Presence and validity of JSON-LD schema for the expected content type, plus Person/Organization authorship markup. |
| 2 | AI-Crawler Access | 12% | Domain | robots.txt rules for AI crawler user-agents; presence of llms.txt (lower weight than v0.1 reflecting limited adoption). |
| 3 | Content Structure | 20% | Page | Chunk density, list/table usage, BLUF answer-first format, paragraph length, heading depth, keyword-stuffing penalty. |
| 4 | Entity Clarity (E-E-A-T) | 18% | Domain | Author `Person` schema, `sameAs` chain, About page, NAP consistency, organisational authority signals. |
| 5 | Technical SEO Baseline | 10% | Page + Domain | Canonical, meta description, title length, sitemap, OG/Twitter cards, plus server-side Core Web Vitals proxies. |
| 6 | Evidence & Citation Readiness | 18% | Page | Statistics/data density, direct quotations with attribution, outbound authoritative citations, internal anchor density, freshness. |

Weights sum to 100. Each sub-score is itself an integer 0–100. The final SageScore is the weighted sum, rounded to integer using banker's rounding for stability across re-audits.

**Why the re-weighting from v0.1.0:**

- **Evidence & Citation Readiness** doubled (10→18%): the Princeton GEO paper shows this is the single highest-impact category (+30–41% citation lift per tactic). v0.1 underweighted it.
- **Entity Clarity / E-E-A-T** up (15→18%): 96% of Google AI Overviews content comes from entities with verifiable author identity, per 2026 benchmarks.
- **AI-Crawler Access** down (20→12%): llms.txt adoption is low (<1000 domains in 2025); major AI companies do not officially use it. Still a positive signal but not dominant.
- **Structured Data** down slightly (25→22%): remains the single largest dimension, but some weight shifts to the E-E-A-T-linked Person/Author schema within Entity Clarity.
- **Technical SEO** and **Content Structure** roughly unchanged.

---

## Per-dimension scoring

### 1. Structured Data — 22%

**Inputs:** All sampled pages.
**Method:** For each page, parse every `<script type="application/ld+json">`. Validate the `@type` against the expected types for that page kind:

| Page kind | Expected schemas |
|---|---|
| Homepage | `Organization` or `WebSite` (required); `BreadcrumbList` (nice-to-have) |
| Article page | `Article` or `BlogPosting` (required); `Person` for author (required for full marks); `BreadcrumbList` (nice) |
| Product page | `Product` with `offers`, `aggregateRating` where applicable |
| FAQ-style page | `FAQPage` (required) |
| HowTo page | `HowTo` (required) |
| Course page | `Course` |
| Event page | `Event` |

Research backing: content with proper schema markup has ~2.5× higher citation rate in AI-generated answers (Stackmatix, 2026).

**Sub-score formula:**

```
base       = (valid expected schemas / total expected schemas) × 80
homepage   = +10 if Organization schema present with `name` and `url`
author     = +10 if all article pages carry Person schema
invalid    = -10 per malformed JSON-LD block, up to -30
sub_score  = clamp(base + homepage + author + invalid, 0, 100)
```

**Findings emitted:** `SCHEMA_ARTICLE_MISSING`, `SCHEMA_PERSON_AUTHOR_MISSING`, `SCHEMA_FAQ_MISSING`, `SCHEMA_ORG_MISSING`, `SCHEMA_PRODUCT_MISSING`, `SCHEMA_INVALID_JSON`, `SCHEMA_MISSING_REQUIRED_PROP`, `SCHEMA_BREADCRUMB_MISSING`.

### 2. AI-Crawler Access — 12%

**Inputs:** `/robots.txt`, `/llms.txt`, `/llms-full.txt`.
**Method:** Boolean checks per AI user-agent, rebalanced for v0.2 against real-world evidence that llms.txt has limited production uptake.

| Component | Points |
|---|---|
| `GPTBot` explicitly allowed (or no rule — default is allow) | 14 |
| `PerplexityBot` allowed | 14 |
| `ClaudeBot` / `anthropic-ai` allowed | 14 |
| `Google-Extended` allowed | 14 |
| `Applebot-Extended` allowed | 10 |
| `Bytespider` / `Amazonbot` allowed | 4 |
| `llms.txt` present and well-formed | 10 |
| `llms-full.txt` present | 5 |
| Dedicated `User-agent: SageScoreBot` rule (any direction — signals awareness) | 2 |
| Wildcard `User-agent: * / Disallow: /` anywhere | -25 |

Sum is clamped to [0, 100].

**Findings emitted:** `AI_CRAWLER_BLOCKED_GPTBOT`, `AI_CRAWLER_BLOCKED_PERPLEXITY`, `AI_CRAWLER_BLOCKED_CLAUDE`, `AI_CRAWLER_BLOCKED_GOOGLE_EXTENDED`, `AI_CRAWLER_BLOCKED_APPLEBOT_EXTENDED`, `AI_CRAWLER_BLOCKED_BYTESPIDER`, `LLMS_TXT_MISSING`, `WILDCARD_DISALLOW`.

### 3. Content Structure — 20%

**Inputs:** All sampled pages.
**Method:** Six sub-checks, weighted.

| Check | Points | Why |
|---|---|---|
| **BLUF / answer-first opening** | 20 | First paragraph (<100 words) contains a complete answer-shaped statement. The Princeton GEO paper found answer-first content is cited disproportionately. |
| **Chunk-size hygiene** | 20 | Sections (content between two same-level headings) average 150–300 words. Content exceeding 300 words shows 31% attention degradation in middle segments per Structural-Feature-Engineering GEO research. |
| **Structural-element ratio** | 20 | Ratio of lists/tables/code-blocks to total content in the 0.25–0.35 band. Structured elements show 43% higher extraction accuracy than equivalent prose. |
| **Paragraph length** | 10 | Mean `<p>` length 30–80 words. Scannable paragraphs per 2026 chunk-optimisation consensus. |
| **Heading depth and validity** | 15 | No `h_n` without `h_(n-1)` above. Overall depth 3–5 levels. Outside band → degraded linearly. |
| **Readability (Flesch)** | 10 | Reading ease ≥ 50 = full; linearly degraded to 0 at Flesch = 20. |
| **Keyword-stuffing penalty** | -5 to 0 | Top repeated non-stopword > 3% of total word count = -5. GEO paper found keyword stuffing has a negative effect on LLM citation. |

Sum clamped to [0, 100].

**Findings emitted:** `CONTENT_BLUF_MISSING`, `CONTENT_CHUNKS_TOO_LONG`, `CONTENT_CHUNKS_TOO_SHORT`, `CONTENT_LOW_STRUCTURAL_ELEMENTS`, `CONTENT_H_HIERARCHY_BROKEN`, `CONTENT_HEADINGS_TOO_FLAT`, `CONTENT_HEADINGS_TOO_DEEP`, `CONTENT_PARAGRAPHS_TOO_LONG`, `CONTENT_READING_EASE_LOW`, `CONTENT_KEYWORD_STUFFING`.

### 4. Entity Clarity (E-E-A-T) — 18%

**Inputs:** Homepage, `/about` (or `/about-us`), footer text, per-article `Person` schema, anchors to author pages.
**Method:** E-E-A-T signals are now the primary filter for AI-Overviews inclusion — 96% of AIO content comes from entities with verifiable author identity (AI-Overviews 2026 benchmarks). Six checks.

| Check | Points |
|---|---|
| `Organization` JSON-LD on homepage with `name`, `url`, `logo` | 20 |
| `sameAs` chain on `Organization`: ≥3 links to LinkedIn, Crunchbase, Wikipedia, GitHub, X, Mastodon, BlueSky, YouTube | 15 |
| `/about` (or `/about-us`) page exists and contains the organisation name | 10 |
| NAP completeness — ≥2 of {phone, email, postal address} discoverable in page text or schema | 10 |
| `Person` schema on article pages with `name` and `url` + author byline links to a bio/about page | 25 |
| Author credentials on bio page — ≥1 of: professional title in `<meta>`, `jobTitle` in Person schema, visible credentials block | 10 |
| Social proof — ≥2 `sameAs` entries on Person schema pointing to LinkedIn / ORCID / Google Scholar / academic profiles | 10 |

Sum clamped to [0, 100].

**Findings emitted:** `ENTITY_ORG_SCHEMA_MISSING`, `ENTITY_ORG_SAMEAS_MISSING`, `ENTITY_ABOUT_MISSING`, `ENTITY_NAP_INCOMPLETE`, `ENTITY_PERSON_SCHEMA_MISSING`, `ENTITY_AUTHOR_BIO_MISSING`, `ENTITY_AUTHOR_CREDENTIALS_MISSING`, `ENTITY_SOCIAL_PROOF_WEAK`.

### 5. Technical SEO Baseline — 10%

**Inputs:** All sampled pages, plus domain-level sitemap reachability and HTML-size proxies for Core Web Vitals.
**Method:** Eight checks (~12.5 points each).

| Check | Notes |
|---|---|
| `<link rel="canonical">` present and matches the page's URL (host match) | |
| `<meta name="description">` length 50–160 chars | |
| `<title>` length 30–65 chars | |
| OpenGraph tags present (`og:title`, `og:description`, `og:type`) | |
| Twitter Card tags present (`twitter:card`, `twitter:title`) | |
| Domain-level: `/sitemap.xml` reachable | piped into every page |
| HTML size under 300 KB (server-side proxy for fast LCP) | |
| ≤2 render-blocking `<script>` tags in `<head>` without `async` or `defer` (proxy for INP) | |

Sum clamped to [0, 100].

We **cannot** measure real Core Web Vitals (LCP, INP, CLS) without rendering JavaScript, which v0.1 does not do (PRD §4). The HTML-size and render-blocking-script checks are deliberate approximations documented as such. If this becomes a blocker post-launch, a headless-browser add-on is a v0.2 conversation.

**Findings emitted:** `TECH_CANONICAL_MISSING`, `TECH_META_DESC_TOO_SHORT`, `TECH_META_DESC_TOO_LONG`, `TECH_TITLE_TOO_LONG`, `TECH_OG_MISSING`, `TECH_TWITTER_MISSING`, `TECH_SITEMAP_UNREACHABLE`, `TECH_HTML_TOO_LARGE`, `TECH_RENDER_BLOCKING_SCRIPTS`.

### 6. Evidence & Citation Readiness — 18%

**Inputs:** All sampled pages. Formerly "Citation-Worthiness" in v0.1; promoted and expanded based on Princeton GEO paper findings.

Princeton GEO measured the effect of specific content tactics on LLM citation visibility:

| Tactic | Measured effect |
|---|---|
| Quotation Addition (quoted passages with attribution) | +41% citation visibility |
| Statistics Addition (specific numbers, percentages, dates) | +40% |
| Cite Sources (outbound citations with author/publication) | +30% |
| Fluency Optimisation | +25% |
| Authoritative writing | +10% |
| Keyword Stuffing | **-10%** |

SageScore's Evidence & Citation Readiness sub-score directly targets these measured levers.

**Five checks:**

| Check | Points | Detection |
|---|---|---|
| **Statistics density** | 20 | ≥3 distinct numeric facts per 1,000 words (digits, %, $, currency, year references). |
| **Direct quotations with attribution** | 20 | ≥1 `<blockquote>` or `<q>` per article-kind page, with an attribution phrase nearby ("according to", "said", named entity link). |
| **Outbound authoritative citations** | 20 | ≥2 distinct outbound links to domains in `pkg/scorer/data/authoritative.txt` per article page. Count scales linearly up to 5. |
| **Internal anchor density** | 15 | Internal links per 1,000 words in the 5–15 sweet spot. Drops linearly outside. |
| **Freshness** | 25 | `dateModified` (JSON-LD) or `Last-Modified` header within 180 days = full 25 points (Perplexity's citation-cliff threshold); within 365 days = 18; within 730 days = 10; older = 0. |

Sum clamped to [0, 100].

Freshness is heavily weighted because Perplexity's citation rate drops to 37% for content older than 180 days (Averi benchmarks, 2026). ChatGPT and Google AI Overviews are more forgiving, but freshness is net positive everywhere.

**Findings emitted:** `EVIDENCE_STATISTICS_THIN`, `EVIDENCE_NO_QUOTATIONS`, `CITATIONS_NO_AUTHORITATIVE_OUTBOUND`, `CITATIONS_THIN_INTERNAL_LINKING`, `CITATIONS_DENSE_INTERNAL_LINKING`, `CITATIONS_STALE_CONTENT`.

---

## Page sampling

For each domain we audit up to 10 pages, deterministically chosen:

1. Always: `/`, `/about` (or `/about-us`), `/robots.txt`, `/sitemap.xml`, `/llms.txt`.
2. **At least 3 article pages**, discovered via:
   - `sitemap.xml` URLs matching `/(blog|articles|posts|news|insights|guides|resources|knowledge)/[^/]+`.
   - `sitemap.xml` URLs with a `<lastmod>` and path depth ≥ 2.
   - Anchors on the homepage pointing to the same patterns.
   - `/blog`, `/articles`, `/posts` index pages and their first 3 outbound article links.
3. Remaining slots: `/contact`, a product/category page if e-commerce, then sitemap diversity picks.

If fewer than 3 articles are found, the audit completes with whatever was found and emits `ARTICLES_INSUFFICIENT_SAMPLE`. The Evidence & Citation Readiness sub-score is then capped at 70.

---

## Roll-up: page → domain

- **Page-level dimensions** (Structured Data, Content Structure, Technical SEO, Evidence & Citation Readiness): computed per page; the domain-level value is a weighted mean across pages with weights `homepage = 2, articles = 1, others = 1`.
- **Domain-level dimensions** (AI-Crawler Access, Entity Clarity): computed once from domain-level inputs.
- **Final SageScore:** `Σ (weight_i × sub_score_i)`, banker's-rounded to integer.

---

## Versioning policy

The scorer carries a SemVer-style version. The version string is stamped onto every audit row and printed at the top of every audit page.

- **Patch bump** (e.g. `0.2.0 → 0.2.1`): bug fixes that don't change scores in stable cases.
- **Minor bump** (e.g. `0.2.0 → 0.3.0`): new findings, new analysers, new sampling rules, threshold tweaks.
- **Major bump** (e.g. `0.2.0 → 1.0.0`): weight changes, dimension changes, semantic redefinitions.

**Old audits are never silently re-scored.** A re-audit (visitor- or owner-triggered) re-fetches the site, re-scores it under the current version, and stamps the new version. Older audits in the cache keep their original score and version until they're re-fetched.

---

## Research base

The weights and sub-checks in v0.2.0 are grounded in peer-reviewed and industry-benchmark sources published 2024–2026. This is not an exhaustive bibliography — just the load-bearing ones.

- **Aggarwal, Murahari, Rajpurohit et al., "GEO: Generative Engine Optimization"** — KDD 2024 / arXiv:2311.09735. Ranks specific content tactics by measured citation lift. Basis for the Evidence & Citation Readiness weight increase and the keyword-stuffing penalty.
- **"Structural Feature Engineering for Generative Engine Optimization"** — arXiv:2603.29979. Basis for chunk-size, heading-depth, and structural-element-ratio thresholds.
- **Averi AI "B2B SaaS Citation Benchmarks Report 2026"** — citation freshness cliffs per platform. Basis for the 180-day weighting in the Freshness sub-check.
- **tryprofound.com "AI Platform Citation Patterns"** — platform-specific source preferences. Informs the multi-platform scoring stance.
- **Stackmatix "Structured Data AI Search Guide 2026"** — schema-markup citation-rate correlation. Basis for the Structured Data weight.
- **Various E-E-A-T / AEO 2026 industry reports** — basis for Entity Clarity promotion and Person/sameAs sub-checks.

The data files `pkg/scorer/data/authoritative.txt` and `pkg/scorer/data/cms-fingerprints.json` are pinned per release; the list versions are part of the scorer version stamp.

---

## What's deliberately excluded from v0.1

Nothing in v0.1 calls an LLM, runs JavaScript, hits a paid API, or estimates traffic / backlinks. These are PRD §5 non-goals and the methodology depends on this stance to remain reproducible from public HTML alone.

Specifically excluded by consequence:

- Real Core Web Vitals (LCP, INP, CLS) — we use HTML-size and render-blocking-script proxies instead.
- JavaScript-rendered content — we score what is present in the initial HTML response only.
- Platform-specific scores — we output one unified score, not separate ChatGPT / Perplexity / AIO numbers. (Candidate for v0.2.)
- Real-time citation tracking via LLM APIs — explicitly out of scope to avoid the crowded "AI visibility tracker" category.

---

## Reproduction

Anyone can reproduce a score using the open-source scorer:

```sh
go install github.com/iserter/sagescore/cmd/sagescore@v0.2.0
sagescore audit example.com -o audit.json
```

The version tag must match the `scorer_version` printed on the audit page for byte-identical reproduction. The `authoritative.txt` and `cms-fingerprints.json` data files are pinned per release.
