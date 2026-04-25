# Decisions Log — SageScore v0.1

**Owner:** Ilyas Serter
**Last updated:** 2026-04-25

This log captures every binding decision for v0.1. Each entry: the question, the decision, the reasoning, and the date.

When a decision is reversed, do not delete the original — append a "Superseded" block with the new decision and the trigger.

---

## D-1 — Pre-generate from seed list, or visitor-triggered only?

**Decision:** Visitor-triggered audits + a founder-curated seed list of 200 well-known domains pre-audited at launch.
**Date:** 2026-04-25.
**Reasoning:** PRD §11 Q1 recommendation. Visitor-triggered is the lower-cost, lower-legal-risk default; the seed list gives Google something to index on day one without us crawling thousands of unrelated sites speculatively. 200 is small enough to manually curate (avoiding controversial / awkward picks) and large enough to seed pSEO indexability.
**Implications:** Phase 4 needs `cmd/sagescore-seed` and `seeds/founder-curated.txt`. No background "crawl-the-web" job exists in v0.1.

---

## D-2 — Favicon and logo on the audit page

**Decision:** Show the audited site's favicon. Do **not** show its logo.
**Date:** 2026-04-25.
**Reasoning:** PRD §11 Q2 recommendation. Favicons are universally treated as fair use for site identification; logos are trademarks and create avoidable liability. Both serve the same recognition goal at the page level.
**Implications:** Audit page template fetches `/<domain>/favicon.ico` (or parses the `<link rel="icon">` from the homepage HTML we already crawled). No image hosting for third-party logos.

---

## D-3 — Public API in v0.1

**Decision:** No public API in v0.1.
**Date:** 2026-04-25.
**Reasoning:** PRD §11 Q3 recommendation. An API adds support burden (key management, rate-limit edge cases, abuse vectors) for marginal launch value. Visitors can use the web UI; OSS users can call the Go library directly. We can ship `/api/v1/score` post-launch once we know what shape callers actually want.
**Implications:** No `/api/*` routes. No API key model in the database. Document the OSS library as the interim API.

---

## D-4 — Open-source the scoring engine

**Decision:** Yes. Publish `pkg/scorer` as an OSS Go library under MIT.
**Date:** 2026-04-25.
**Reasoning:** PRD §11 Q4 recommendation (open-source yes). License choice retained as **MIT** to match the repository's original `LICENSE` file. Apache-2.0 was considered for its patent grant, but MIT is simpler, already on file, and fine for a methodology-transparency repo where the risk surface is documentation rather than patented algorithms.
**Implications:** `pkg/scorer/README.md` is OSS-quality. The closed-source surface is `pkg/web` and `pkg/store` only; they live in the same repo for now and we'll split if/when there's a reason.

---

## D-5 — Authoritative-domains list

**Decision:** Static, versioned text file at `pkg/scorer/data/authoritative.txt`. ~200 hand-curated domains.
**Date:** 2026-04-25.
**Reasoning:** A bundled file is deterministic, reviewable in PRs, and avoids any ML/feature dependency. The list grows by PR, not by background process. Versioned via git history; the list version is part of the scorer version stamp.
**Implications:** The list is checked into the repo and embedded via `embed.FS`. Updates to the list trigger a scorer version bump.

---

## D-6 — CMS detection method

**Decision:** Hand-curated 12-fingerprint set in `pkg/scorer/data/cms-fingerprints.json`. Header + meta + path heuristics. No third-party dependency (e.g. Wappalyzer).
**Date:** 2026-04-25.
**Reasoning:** CMS detection in v0.1 is used only for the "What is AEO" content variant on the audit page, not for scoring. A 12-entry hand-curated set is enough to cover ~90% of the long tail (WordPress, Shopify, Webflow, Squarespace, Wix, Next.js, Gatsby, Hugo, Ghost, Drupal, Joomla, "unknown" fallback). Anything fancier is wasted effort.
**Implications:** New CMS variants ship as PRs to both the JSON file and the matching `content/cms-*.md` fragment.

---

## D-7 — Email provider for removal-confirmation flow

**Decision:** Resend, EU region.
**Date:** 2026-04-25.
**Reasoning:** Resend has an EU sending region (data-residency win for GDPR), a clean Go SDK, and free-tier limits that easily cover v0.1's expected removal volume (PRD §9: <2% of audits). Postmark would also work but is more expensive and US-hosted by default.
**Implications:** `pkg/email/resend.go` is the only mailer. Sender domain: `noreply@sagescore.org`. SPF, DKIM, DMARC records to be configured in Phase 4.

---

## D-8 — License: MIT (full repo)

**Decision:** The whole repository is MIT-licensed.
**Date:** 2026-04-25.
**Reasoning:** Splitting the license between OSS and closed parts of the same repo is a maintenance burden that doesn't buy anything. MIT is the license already on file and is the simplest permissive option. The web layer, templates, and migrations are inseparable from the scorer in practice; a single license keeps things simple. We can re-license a derivative product later if needed.
**Implications:** `LICENSE` stays as MIT across the whole repo. Any future commercial fork/spinout will be a separate repo with its own license.

---

## D-9 — Module path and Go version

**Decision:** Module path `github.com/iserter/sagescore`. Minimum Go version: 1.24.
**Date:** 2026-04-25.
**Reasoning:** GitHub username matches; vanity import path is unnecessary for v0.1. Go 1.24 is the current toolchain on the dev machine and brings standard-library improvements (e.g. `sync/atomic` enhancements, generic improvements) that we benefit from at zero cost.
**Implications:** `go.mod` declares `go 1.24`. CI pins the same version.

---

## D-10 — ORM: GORM v2

**Decision:** Use GORM v2 (`gorm.io/gorm`) for all persistence; both SQLite and Postgres drivers in `gorm.io/driver/{sqlite,postgres}`.
**Date:** 2026-04-25.
**Reasoning:** Documented in Technical Plan §5.1. Single-line driver swap, struct-tag schema kept beside domain types, AutoMigrate in v0.1 with goose-based SQL migrations as an escape hatch. `ent` and `sqlc` rejected for v0.1 reasons in the Technical Plan.
**Implications:** `pkg/store` exposes a repository interface; GORM is the implementation behind it. Tests use in-memory SQLite (`:memory:`).

---

## D-11 — Methodology bumped to v0.2.0 (evidence-informed re-weighting)

**Decision:** Replace the v0.1.0 methodology with v0.2.0 before any public audit is produced. See `docs/methodology.md` for the full spec.
**Date:** 2026-04-25.
**Reasoning:** The v0.1.0 weights were authored from first principles before reviewing the empirical literature. A research review surfaced four signals that warrant substantive re-weighting:

1. **Princeton GEO paper** (Aggarwal et al., KDD 2024): measured +41% citation lift from quotations, +40% from statistics, +30% from cited sources, **-10% from keyword stuffing**. Evidence-density tactics are the highest-impact GEO lever, yet v0.1.0 had the "Citation-Worthiness" dimension at only 10%.
2. **2026 Google AI Overviews benchmarks**: 96% of AIO content comes from entities with verifiable author identity. Entity Clarity (E-E-A-T) was underweighted at 15%.
3. **llms.txt adoption data**: <1000 domains published llms.txt by mid-2025; major AI companies (OpenAI, Google, Anthropic, Meta) do not officially use it. v0.1.0 gave llms.txt 25 points inside AI-Crawler Access — overweighted.
4. **Structural Feature Engineering GEO paper** (arXiv:2603.29979): measured 43% higher extraction accuracy from lists/tables/code blocks vs prose; 31% attention degradation in chunks >300 words. These specific, measurable structural signals were absent from v0.1.0.

**Changes:**

| Dimension | v0.1 | v0.2 | Delta |
|---|---|---|---|
| Structured Data | 25% | 22% | -3 |
| AI-Crawler Access | 20% | 12% | -8 (llms.txt overweight) |
| Content Structure | 20% | 20% | 0 (sub-checks reshaped) |
| Entity Clarity (E-E-A-T) | 15% | 18% | +3 |
| Technical SEO | 10% | 10% | 0 (CWV proxies added) |
| Citation-Worthiness → Evidence & Citation Readiness | 10% | 18% | +8 (Princeton GEO) |

Plus a new keyword-stuffing penalty inside Content Structure, `Person` schema checks inside Structured Data and Entity Clarity, and HTML-size/render-blocking-script proxies inside Technical SEO.

**Implications:** Phase 1 analyser specs in `tasks/phase-1-engine.md` updated to match. Technical Plan §3.2 weights table updated. `pkg/scorer/scorer.go` constant `DimCitationWorthy` renamed to `DimEvidenceCitation`; `Weights` map updated; `Version` bumped to `0.2.0-dev`. No audits have been produced yet, so no migration is required.
