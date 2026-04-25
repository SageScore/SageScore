# Technical Plan — SageScore v0.1

**Companion to:** `docs/PRD.md`
**Owner:** Ilyas Serter
**Date:** 2026-04-25
**Target ship:** Public beta 2026-06-30
**Build window:** ~24 working days (May 15 – Jun 30, 2026)

---

## 1. Architectural overview

SageScore is a single Go binary that exposes two surfaces from one codebase:

1. A **scorer library** (`pkg/scorer`) that, given a domain, produces a deterministic 0–100 SageScore plus per-dimension findings.
2. A **web service** (`cmd/sagescore-web`) that serves `sagescore.org/<domain>`, the homepage, methodology, privacy, and removal flows. It calls the scorer library, persists results to SQLite, and renders server-side HTML.

A separate thin **CLI** (`cmd/sagescore`) wraps the scorer library for OSS users and internal seeding.

```
                       ┌────────────────────────────────────────────────┐
                       │  Go binary (single static)                     │
                       │                                                │
  HTTP request ───►    │  chi router ──► handlers ──► scorer.Audit()    │
  (visitor / cron)     │                              │                 │
                       │                              ▼                 │
                       │                       crawler (colly)          │
                       │                              │                 │
                       │                              ▼                 │
                       │                       analysers (6 sub-scores) │
                       │                              │                 │
                       │                              ▼                 │
                       │                       SQLite cache             │
                       │                                                │
                       │  html/template  ◄────────────┘                 │
                       └────────────────────────────────────────────────┘
                                          │
                                          ▼
                                 Cloudflare (cache+WAF)
```

The whole thing is one process, one binary, one SQLite file, one VPS — per the PRD's cost and ops constraints (§7).

---

## 2. Repository layout

```
sagescore/
├── cmd/
│   ├── sagescore/          # CLI: `sagescore audit example.com`
│   └── sagescore-web/      # web server entrypoint
├── pkg/
│   ├── scorer/             # public OSS-able scoring library
│   │   ├── scorer.go       # orchestrator: Audit(domain) -> Result
│   │   ├── crawl/          # colly wrapper, robots.txt, rate limiter
│   │   ├── fetch/          # plain HTTP GET, redirects, retries
│   │   ├── parse/          # HTML, JSON-LD, OG, sitemap.xml
│   │   ├── analyse/        # one file per sub-score
│   │   │   ├── schema.go
│   │   │   ├── ai_crawlers.go
│   │   │   ├── content.go
│   │   │   ├── entity.go
│   │   │   ├── tech_seo.go
│   │   │   └── citations.go
│   │   └── score.go        # weights, aggregation, 0-100 mapping
│   ├── store/              # SQLite repository (audits, removals, owners)
│   ├── render/             # html/template helpers, content templates
│   └── web/                # handlers, middleware, routes
├── migrations/             # goose-style SQL migrations
├── templates/              # *.gohtml server-rendered pages
├── static/                 # css, fonts, og images
├── content/                # CMS-detection-based copy fragments (markdown)
├── deploy/                 # Dockerfile, fly.toml / hetzner systemd unit
├── docs/                   # PRD.md, Technical_Plan.md, methodology.md
└── go.mod
```

Rationale: the `pkg/scorer` boundary is the OSS surface (PRD §11 Q4). `pkg/web` and `pkg/store` stay closed-source-friendly without creating a hard split.

---

## 3. Scoring engine

### 3.1 Result model

The score is computed **per page** first, then rolled up to a domain score. Per-page records are persisted so the audit page can show "your weakest article" and so re-audits can diff individual URLs.

```go
type Audit struct {
    Domain      string
    FetchedAt   time.Time
    Score       int                 // 0-100, weighted roll-up of pages + domain-level signals
    Subscores   map[Dimension]Sub   // 6 dimensions, domain-level aggregate
    Pages       []PageAudit         // sampled URLs: homepage + about + ≥3 articles + others (≤10 total)
    Robots      RobotsSummary       // domain-level (single fetch)
    Sitemap     SitemapSummary      // domain-level (single fetch)
    CMS         string              // detected: WordPress, Shopify, Webflow, Next.js, "unknown"
    Errors      []string            // non-fatal (timeouts, 5xx, etc.)
}

type PageAudit struct {
    URL         string
    Kind        PageKind            // "homepage" | "about" | "article" | "product" | "other"
    Score       int                 // 0-100, weighted across the 4 page-level dimensions
    Subscores   map[Dimension]Sub   // schema, content, tech_seo, citations (page-level)
    WordCount   int
    FetchedAt   time.Time
    StatusCode  int
}

type Sub struct {
    Score    int                    // 0-100
    Findings []Finding              // {Severity, Code, Message, Evidence, FixCTA}
}
```

`Finding.Code` (e.g. `SCHEMA_ARTICLE_MISSING`) drives both the rendered explainer and the upsell mapping (PRD §2 job 3).

**Roll-up rules:**
- Domain-level dimensions (`ai_crawlers`, `entity_clarity`) come from a single fetch each (`/robots.txt`, `/llms.txt`, homepage, `/about`).
- Page-level dimensions (`schema`, `content`, `tech_seo`, `citations`) are computed per page; the domain-level value is the **weighted mean** across pages, with the homepage weighted 2× and articles weighted 1×.
- A domain with fewer than 3 articles found is still scored, but the audit page surfaces a finding `ARTICLES_INSUFFICIENT_SAMPLE` and the citation-worthiness sub-score is capped at 70 (you cannot prove citation-worthiness without article-shaped content).

### 3.2 Sub-score implementations

| Dimension | Inputs | Method | Weight |
|---|---|---|---|
| Structured data | Sampled pages | Parse all `<script type="application/ld+json">`, validate types against schema.org expected for detected page kind (homepage → `Organization`/`WebSite`; blog → `Article`; product → `Product`; FAQ → `FAQPage`). Score = % of expected schemas present and valid. | 25% |
| AI-crawler access | `/robots.txt`, `/llms.txt`, `/llms-full.txt` | Boolean checks for `GPTBot`, `PerplexityBot`, `ClaudeBot`, `Google-Extended`, `Applebot-Extended`. Penalise wildcard disallow, reward explicit allow + presence of `llms.txt`. | 20% |
| Content structure | Sampled pages | h-tag tree validity, mean paragraph length, BLUF heuristic (first sentence under 200 chars and contains a noun + verb), Flesch reading ease. | 20% |
| Entity clarity | Homepage + `/about` + footer | Author/Org schema, About page presence, NAP regex (phone, email, address), `sameAs` social links, `<meta name="author">`. | 15% |
| Technical SEO | Sampled pages | `<link rel="canonical">`, `<meta name="description">` 50–160 chars, `<title>` 30–65 chars, sitemap reachable, OG + Twitter cards. | 10% |
| Citation-worthiness | Sampled pages | Outbound link analysis (domains classified via embedded list of authoritative TLDs/domains), internal anchor density, `dateModified` or last-mod header recency. | 10% |

Final score: `round(Σ weight_i × sub_i)`. Tie-broken to integer with banker's rounding for stability across re-audits.

### 3.3 Determinism

The scorer must be **deterministic for a fixed input** so that the methodology page (§6 Defensive language) is defensible. Concretely:
- No randomness in URL sampling — sort sitemap URLs lexicographically, take first N matching each kind.
- No floating-point comparisons in branching logic.
- Pin the authoritative-domains and CMS-fingerprints lists; version them and stamp the version into the `Audit` record.

### 3.4 Crawler

- Library: `gocolly/colly v2`.
- User-Agent: `SageScoreBot/0.1 (+https://sagescore.org/bot; contact@sagescore.org)` (PRD §6).
- `colly.Async(true)`, `LimitRule{Parallelism: 1, Delay: 5s}` per host (PRD §6).
- `colly.RobotsTxt = true`. If `SageScoreBot` is disallowed → return `OptOutResult` (no audit, page renders the opt-out template).
- Hard caps: max 10 pages per domain, 30s per page, 120s total per audit, 5MB per response. (Total bumped from 90s to accommodate the article minimum at 5s/page rate limit.)
- Retries: 1 retry on network error, none on 4xx, 1 retry on 5xx with 5s backoff.
- No JS rendering. No cookies. No POST. No auth headers.

**URL sampling — must yield ≥ 3 article pages:**

1. Always fetch: `/`, `/about` (or `/about-us`), `/robots.txt`, `/sitemap.xml`, `/llms.txt`.
2. **Article discovery**, in priority order, stopping when 3 articles are found:
   - URLs in `sitemap.xml` whose path matches `/(blog|articles|posts|news|insights|guides|resources|knowledge)/[^/]+`.
   - URLs in `sitemap.xml` with a `<lastmod>` and a path depth ≥ 2 that are not in the always-fetch list.
   - Anchors discovered on the homepage that point to same-host URLs matching the patterns above.
   - `/blog`, `/articles`, `/posts` index pages (then their first 3 outbound article links).
3. Article URLs are sorted by sitemap order (or lex order when no sitemap), then the first 3 unique are taken — this preserves determinism (§3.3).
4. Remaining slots up to 10 fill with: a `/contact`, a product/category page if e-commerce CMS detected, then sitemap diversity picks.
5. If fewer than 3 articles are discoverable after exhausting the above, the audit completes with whatever was found and emits the `ARTICLES_INSUFFICIENT_SAMPLE` finding.

A page is classified `article` when (a) it was found via the article-discovery pipeline above **and** (b) its parsed HTML contains either an `Article`/`BlogPosting` JSON-LD or an `<article>` element with ≥ 300 words. Otherwise it is reclassified as `other`.

### 3.5 Concurrency model

- Per-audit: bounded by per-host rate limit (1 req / 5s).
- Server-wide: a `chan struct{}` semaphore of size 50 (PRD §10 server overload). Audits that would exceed it queue with a 60s timeout and return HTTP 503 + Retry-After.
- Worker pool of 10 goroutines pulling audit jobs from an in-process queue. Synchronous on-demand audits short-circuit the queue when capacity is available; otherwise enqueue and return a "we're computing this, refresh in 30s" interstitial.

---

## 4. Web service

### 4.1 Routes

| Method | Path | Handler | Notes |
|---|---|---|---|
| GET | `/` | `homeHandler` | Single input, CTA. |
| POST | `/audit` | `submitHandler` | Validates domain, redirects to `/<domain>`. |
| GET | `/{domain}` | `auditHandler` | Cache-first; computes if missing/stale. Server-rendered. |
| GET | `/about` | static | Methodology, weights, governance. |
| GET | `/methodology` | static | Detailed explainer, OSS link. |
| GET | `/privacy` | static | GDPR policy. |
| GET | `/remove` | `removeFormHandler` | Owner removal request form. |
| POST | `/remove` | `removeSubmitHandler` | Sends email-verification link. |
| GET | `/remove/confirm` | `removeConfirmHandler` | Token-verified; flips `noindex` immediately, schedules delete. |
| GET | `/recrawl/{token}` | `recrawlHandler` | Owner-signed re-audit link from the audit page. |
| GET | `/sitemap.xml` | `sitemapHandler` | Generated from audited domains where `noindex=false`. |
| GET | `/robots.txt` | static | Allows everything; disallows `/remove`, `/recrawl/*`. |
| GET | `/bot` | static | Public crawler-info page (matches UA string). |
| GET | `/healthz` | trivial | For uptime monitor. |

Domain validation: parse with `net/url`, lowercase host, reject IPs, reject anything not matching `^[a-z0-9.-]+\.[a-z]{2,}$`, reject our own domain, reject blocked-list (porn TLDs, known illegal content lists) to avoid hosting unsavoury landing pages.

### 4.2 Caching policy

- **First-party cache (SQLite):** every audit row has `fetched_at`. Considered fresh for 30 days (PRD §6).
- **Stale-while-revalidate:** if 30 < age ≤ 60 days, serve cached + enqueue background re-audit.
- **CDN (Cloudflare):** `Cache-Control: public, max-age=86400, stale-while-revalidate=604800` on `/{domain}` when the audit is fresh; `no-store` on `/remove`, `/audit`, `/recrawl/*`.
- **Owner re-audit:** clears both caches via Cloudflare API on confirm.

### 4.3 Rendering

- `html/template` with a base layout + per-page partials.
- Each `/{domain}` page targets ≥ 800 unique words (PRD §10):
  - Hero (~80 words, fixed phrasing with substitutions)
  - Score breakdown table (varies by findings)
  - Per-dimension findings prose, generated from `Finding.Code` → markdown templates with the evidence inlined (this is the bulk of unique content).
  - **Per-page breakdown table** listing every sampled URL with its individual score, kind (article/homepage/about/etc.), and the top finding for that page. The 3 articles are the visual anchor of this section — they're what an AEO engineer would dig into first.
  - "What is AEO" section that **varies by detected CMS** — six variants in `content/cms-*.md` (WordPress, Shopify, Webflow, Squarespace, Next.js, generic).
  - Methodology footer.
  - Upsell CTA mapped from highest-severity finding to the matching SAGE GRIDS plugin (e.g. `SCHEMA_FAQ_MISSING` → SG Product Posts Generator).
  - Removal/recrawl link block.
- Open Graph image: pre-rendered SVG → PNG at audit-write time, stored in `static/og/<domain>.png`. Score in giant numerals; favicon top-left.

### 4.4 Indexability rules

- `<meta name="robots" content="index,follow">` only when **all** of:
  - `SageScoreBot` is allowed in the audited site's robots.txt **at fetch time**.
  - Word count of rendered content ≥ 800.
  - No active removal request.
  - Audit completed (not the "computing" interstitial).
- Otherwise `noindex,nofollow`.
- Domain-level `noindex` flag in DB overrides everything.

---

## 5. Data model

### 5.1 ORM choice

We use **GORM v2** (`gorm.io/gorm`) with the SQLite driver in v0.1 and the Postgres driver post-v0.1. GORM is the pragmatic pick because:

- Native dialect drivers for both SQLite and Postgres — switching is a one-line change in `pkg/store/db.go` (`sqlite.Open(...)` → `postgres.Open(...)`).
- AutoMigrate-style schema management for v0.1 plus first-class compatibility with raw SQL migrations (we'll use `pressly/goose` for any non-trivial migration that AutoMigrate can't express, e.g. partial indexes).
- Struct-tag schema definitions kept alongside the domain types in `pkg/store/models.go`, which means the data model stays in one file and is reviewable in PRs.

Alternatives evaluated and rejected for v0.1:
- **`ent`** — schema-first with code generation; cleaner long-term but heavier to set up and the codegen workflow adds friction during the early iteration weeks.
- **`sqlc`** — compile-time-checked SQL is appealing but the SQLite and Postgres dialects diverge enough that we'd be maintaining two sets of `.sql` files; defeats the "easy switch" goal.
- **Raw `database/sql` + a thin repository** — the original plan; rejected because it forces us to hand-write dialect-specific SQL for things like upserts and partial indexes.

The `pkg/store` package exposes a repository interface (`AuditRepo`, `RemovalRepo`, etc.) that the rest of the app depends on; GORM is an implementation detail behind it. This keeps the app testable and gives us an exit hatch if GORM ever becomes the wrong fit.

### 5.2 Storage layout

SQLite (v0.1), one file at `/var/sagescore/db.sqlite`, WAL mode. Same models work unchanged on Postgres.

```go
// pkg/store/models.go (sketch)

type Audit struct {
    Domain         string    `gorm:"primaryKey;size:253"`
    Score          int       `gorm:"not null"`
    ResultJSON     string    `gorm:"type:text;not null"`     // full scorer.Audit
    FetchedAt      time.Time `gorm:"not null;index"`
    ScorerVersion  string    `gorm:"not null;size:32"`
    Noindex        bool      `gorm:"not null;default:false"`
    OptOut         bool      `gorm:"not null;default:false"`
    WordCount      int       `gorm:"not null"`
    Pages          []PageAudit `gorm:"foreignKey:Domain;references:Domain;constraint:OnDelete:CASCADE"`
}

type PageAudit struct {
    ID             uint64    `gorm:"primaryKey"`
    Domain         string    `gorm:"not null;index;size:253"`
    URL            string    `gorm:"not null;size:2048"`
    URLHash        string    `gorm:"not null;size:64;uniqueIndex:idx_page_domain_url"` // SHA-256(URL), composite-unique with Domain
    Kind           string    `gorm:"not null;size:16"`       // homepage|about|article|product|other
    Score          int       `gorm:"not null"`
    SubscoresJSON  string    `gorm:"type:text;not null"`     // map[Dimension]Sub
    WordCount      int       `gorm:"not null"`
    StatusCode     int       `gorm:"not null"`
    FetchedAt      time.Time `gorm:"not null"`
    ScorerVersion  string    `gorm:"not null;size:32"`
}

type RemovalRequest struct {
    ID            uint64    `gorm:"primaryKey"`
    Domain        string    `gorm:"not null;index;size:253"`
    Email         string    `gorm:"not null;size:320"`
    Token         string    `gorm:"not null;uniqueIndex;size:64"` // HMAC hex
    RequestedAt   time.Time `gorm:"not null"`
    ConfirmedAt   *time.Time
    DeletedAt     *time.Time                                  // 30-day SLA marker
    IPHash        string    `gorm:"not null;size:64"`         // SHA-256(ip + daily salt)
}

type RecrawlToken struct {
    Token     string    `gorm:"primaryKey;size:64"`
    Domain    string    `gorm:"not null;index;size:253"`
    ExpiresAt time.Time `gorm:"not null"`
}

type RateLimit struct {
    IPHash      string    `gorm:"primaryKey;size:64"`
    Bucket      string    `gorm:"primaryKey;size:16"`         // audit|remove|api
    WindowStart time.Time `gorm:"primaryKey"`
    Count       int       `gorm:"not null"`
}
```

Notes:

- `Audit` ↔ `PageAudit` is 1:N. Pages are written in the same transaction as the parent audit and cascade-delete on owner removal.
- `URLHash` lets us index unique-per-domain URL without paying for an index on a 2KB column.
- `ResultJSON` and `SubscoresJSON` keep the rendering path simple (one read → unmarshal → render) while still letting us query by `Score`, `Domain`, `FetchedAt` directly. If we ever need analytical queries, a Postgres view over JSONB will be straightforward.
- Migrations: `db.AutoMigrate(...)` on startup for v0.1; once a migration is non-trivial we move it to a goose `.sql` file under `migrations/`, with separate `*.sqlite.sql` / `*.postgres.sql` only when a dialect actually requires it.

### 5.3 SQLite → Postgres switch path

1. Set `SAGESCORE_DB_DRIVER=postgres` and `SAGESCORE_DB_DSN=...`.
2. `pkg/store/db.go` selects the driver by env var; no other code changes.
3. Backfill: `litestream` snapshot → `pgloader` one-shot transfer (well-trodden path, an hour of work).
4. Re-enable AutoMigrate / run any goose migrations on the Postgres target.

The repository interface stays identical, so handler code is unchanged.

---

## 6. Privacy, compliance, removal flow

### 6.1 Removal flow (PRD §6)

1. Owner POSTs `/remove` with `domain` + contact email.
2. Server immediately sets `audits.noindex = 1` and serves a "removal pending" page (no audit content), returns `Cache-Control: no-store`, purges Cloudflare for `/{domain}`.
3. Server emails a confirmation token to the supplied address.
4. On `/remove/confirm?token=…`, mark `confirmed_at = now()`. The page now stays `noindex` permanently.
5. A nightly job (`cmd/sagescore-web --reap`) hard-deletes `audits` rows whose `removal_requests.confirmed_at` is older than 30 days. The deleted-at timestamp is retained on the removal_requests row for audit trail, but no audit content remains.

The form is one field (the email) — domain is bound via the page URL. No login. No account.

### 6.2 Anti-abuse on removal

- Email verification: nobody can deindex a third party's audit without inbox access *to a domain-related address*. Where possible we accept only `@<domain>` or common admin variants (`hostmaster@`, `webmaster@`, `postmaster@`); otherwise we'll accept any email but flag for manual review and only act after the owner clicks the link.
- Per-IP-hash rate limit: 5 removal requests per IP per day.

### 6.3 GDPR

- Documented legitimate-interest assessment in `docs/lia.md` before public launch.
- No personal data is collected from audited sites — assert in privacy policy.
- For removal-request data (email, IP hash), the privacy policy lists purpose, retention (30 days post-deletion for audit trail then purged), and the data-subject rights.
- Plausible self-hosted; no cookie banner needed (no first-party cookies set on visitors).

### 6.4 Defensive language

A linter check in CI (`scripts/lint-content.go`) scans all templates and content fragments for blocked words: "bad", "broken", "neglected", "lazy", "incompetent", "outdated" used about the site. Build fails if found in any user-facing template.

---

## 7. SEO / pSEO mechanics

Per PRD §10 — thin pSEO is the #1 ranking risk.

- **Unique content floor:** 800 words verified at render time. If under, page is `noindex` and shows a "this audit is too thin to publish" banner.
- **Internal linking:** every audit page links to 5 sibling audits in `/sitemap.xml`, chosen by:
  - 2 with similar score band (±5)
  - 2 in the same TLD
  - 1 randomly from the seed set
- **Sitemap:** auto-regenerated on audit-write, capped at 50,000 URLs, split into multiple files when needed.
- **Canonical:** `https://sagescore.org/<lowercased-host>` always.
- **Schema.org on our own pages:** `WebPage` + `Dataset` + `BreadcrumbList`.
- **Seed campaign:** 200 hand-picked well-known domains pre-audited at launch (PRD §11 Q1).

---

## 8. Cold-outreach integration

The audit URL is the payload the SG Lead Manager sends. The audit page therefore needs:

- A stable URL pattern (`sagescore.org/<domain>`).
- A short OG image preview (~600 KB max) so the link unfurls on LinkedIn/email clients.
- A `?ref=outreach` query parameter that we pass through to Plausible (custom event) and use to show a slightly different hero CTA targeted at the audited owner ("Got this in an email? Here's what your prospects see").

We don't store who-emailed-whom in SageScore. The Lead Manager owns that.

---

## 9. Observability

- Logs: structured (`slog` with JSON handler) to stdout, captured by systemd or Fly.io.
- Metrics: in-process Prometheus registry, exposed on a separate port behind firewall. Counters: `audits_started_total`, `audits_failed_total{reason}`, `removals_requested_total`, `cache_hits_total`. Histograms: audit duration, fetch duration.
- Alerts (cheap, pragmatic): one BetterStack uptime probe on `/healthz`; one alert when removals/day > 2% of audits/day (PRD §9 success metric).
- Error tracking: Sentry free tier.

---

## 10. Deployment

- Single Docker image, multi-stage Go build, ~30MB.
- Hetzner CX22 VPS (€4.85/mo) running Caddy → Go binary on `:8080`.
- Caddy handles TLS (Let's Encrypt) and HTTP→HTTPS redirect.
- Cloudflare in front for DDoS + edge cache.
- SQLite file backed up nightly via `litestream` to a Hetzner Object Storage bucket. RPO ≤ 1 min, RTO ≤ 10 min.
- Deploy: `git push` → GitHub Action builds and pushes the image, then `ssh` deploy script pulls and restarts the systemd unit. Zero-downtime via systemd socket activation if needed; otherwise ~2s downtime is acceptable for v0.1.

Total monthly cost target: < €30 (PRD §9). Expected: €4.85 VPS + €0 Cloudflare free + €0 Plausible (self-hosted on same VPS) + €11/yr domain + €1 storage ≈ €6/month. Headroom is the point.

---

## 11. Testing strategy

- **Unit tests** per analyser, with HTML fixtures committed under `pkg/scorer/testdata/`. One fixture per finding code.
- **Golden tests** for the full scorer: 10 hand-picked real domains snapshotted at a fixed date; their full `Audit` JSON is committed. CI uses recorded HTTP responses (`go-vcr`) so the tests are hermetic.
- **Property tests** on the scoring aggregation: monotonic in each sub-score, total ∈ [0,100].
- **Integration test** against a tiny in-repo HTTP server that serves a synthetic site with known issues.
- **Content-lint** (§6.4) runs in CI.
- **End-to-end** smoke: a `make e2e` target that boots the binary, runs an audit against a local fixture site, and asserts the rendered page contains expected findings + correct `<meta robots>`.

No mocked HTTP in scorer tests beyond the VCR layer — we want the real parser path exercised.

---

## 12. Phased build (mapped to PRD §12)

| Phase | Deliverable | Key tasks | Window |
|---|---|---|---|
| 0 — Decisions | Brief approved, methodology locked, privacy + LIA drafted, removal flow specified | Resolve PRD §11 open questions; lock weights; draft `docs/methodology.md`, `docs/lia.md`, `/privacy` copy. | 2 days |
| 1 — Engine | Go scorer producing JSON for any URL | `pkg/scorer` skeleton, crawler+robots, all 6 analysers, score aggregation, CLI, golden tests, OSS-ready README. | 1 week |
| 2 — Web service | `sagescore.org/<domain>` live, single audit on demand | chi routing, SQLite store, on-demand audit + queue, server-rendered audit page, OG image generation, healthz. | 1 week |
| 3 — pSEO fitness | Sitemap, schema, internal linking, methodology, privacy, removal | Sitemap generator, schema.org on our pages, 6 CMS-variant content fragments, `/about`, `/methodology`, `/privacy`, `/remove` flow with email verification, content lint, indexability gating. | 1 week |
| 4 — Soft launch | Submit to GSC, seed 200 audits, Hacker News post | Submit sitemap, run seed-audit script over hand-picked list, set up Plausible + uptime probe, post on HN ("Show HN: I built an open-source AEO scorer"). | 3 days |

Critical-path order inside Phase 1: robots+crawler → AI-crawler analyser (independent of HTML) → schema → tech SEO → content → entity → citations. This sequence keeps each PR small and lets the CLI ship after step 4 of 6 sub-scores if needed.

---

## 13. Risks & technical mitigations

| Risk | Technical mitigation |
|---|---|
| Thin-content deindex | 800-word floor enforced at render; CMS-varied copy; 5 internal sibling links per page; canonical + sitemap hygiene. |
| Crawl-induced complaint from a site owner | Strict `colly` rate limits, `robots.txt` honoured, identifiable UA, `/bot` info page, instant opt-out path. |
| Defamation claim | Score is a pure function of public HTML + a versioned, public weights table; the methodology page links to the OSS scorer at the version that produced this audit. |
| Score drift across versions | `scorer_version` stamped on each `audits` row; old audits are NOT silently re-scored — re-crawl rewrites both content and version. |
| SQLite write contention under viral load | WAL mode; writes serialised through a single goroutine; reads concurrent; Cloudflare absorbs read fan-out. |
| Cost blow-up | Hard caps: 50 concurrent audits, 10 pages/audit, 5MB/response, 90s/audit. The binary cannot exceed a known compute envelope. |
| GDPR investigation | LIA on file before launch; no PII from third parties; minimal first-party data; one-form removal; documented retention. |

---

## 14. Open technical decisions that block coding

These map to PRD §11 plus a few engine-level ones.

1. **OSS license for `pkg/scorer`** — recommend Apache-2.0 (patent grant matters for a "methodology defence" repo).
2. **Email sending** — Resend vs. Postmark for the removal-confirmation flow. Recommend Resend (cheaper, EU region available).
3. **Authoritative-domains list** — bundle a static list (Wikipedia, gov, edu, major news) or compute from a small embedded ML signal? Recommend static, versioned, in `pkg/scorer/data/authoritative.txt`.
4. **CMS detection** — header/meta heuristics only, or include a bundled fingerprint set (Wappalyzer-style)? Recommend a hand-curated 12-fingerprint set; we only need it for the copy variant, not for scoring.
5. **`?ref=outreach` design** — keep it as a non-cached cookie-less query param so caching stays clean; render variant via template branch only.

---

## 15. What we are explicitly NOT building in v0.1

Mirrors PRD §5 plus engine-level non-goals:

- No JS rendering, no headless browser.
- No LLM API calls anywhere in the audit path (PRD §4).
- No backlink/traffic data, no third-party paid integrations.
- No public API in v0.1 (decision recommended in PRD §11 Q3).
- No accounts, no logins, no dashboards. The only persistent identity in the system is "domain".
- No comparison pages, no time-series, no page-level (sub-domain) audits.

Reintroducing any of these is a v0.2 conversation, not a v0.1 scope-creep.
