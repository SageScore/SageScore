# Product Brief — SageScore Public Audit Pages

**Document:** SageScore v0.1 — Public Per-Domain Audit Pages
**Owner:** Ilyas Serter
**Date:** April 25, 2026
**Status:** Draft — not before the April 21 launch hits ≥50 founding sales
**Build window:** Earliest start, May 15, 2026; target public beta, June 30, 2026

---

## 1. One-line product definition

`sagescore.org/<domain>` displays a free, public, AEO-readiness audit of any website — a 0–100 score plus prioritised fixes — generated entirely from public on-page signals, with no LLM API calls and no scraping of personal data.

## 2. Why this exists (the strategic role)

Three jobs in one asset:

1. **Distribution moat.** Each public audit page is an indexed pSEO landing page. The long-tail keyword universe ("`<competitor> SEO audit`", "`is <site> optimised for ChatGPT`") is enormous and uncrowded for AEO-flavoured queries.
2. **Cold-outreach payload.** "Your site scored 31/100 on AI-search readiness — public audit at sagescore.org/yoursite.com" replaces "Hi, I built a tool" in every cold email going out from SG Lead Manager. This is the §6.4 *Personalised Audit Outreach Engine* operationalised.
3. **Upsell funnel into SAGE GRIDS plugins.** Every audit ends with "Pages failing FAQ schema → fix automatically with SG Product Posts Generator" → CTA → checkout.

## 3. ICP & primary user flows

- **Discovery user (organic):** types competitor's site into Google, lands on `sagescore.org/<their-competitor>.com`, sees the score, runs their own site. Goal: convert to email signup or tool buyer.
- **Cold-email recipient:** clicks the link in the email, sees their own audit. Goal: buy or book a call.
- **Owner (defensive):** finds their own page, wants to remove or improve. Goal: provide a frictionless removal path *and* a 1-click "fix with SAGE GRIDS" path.

## 4. Scope — what's in v0.1

| Surface | What it does |
|---|---|
| `sagescore.org/<domain>` | Public audit page, server-rendered HTML, indexable by default *only* for domains where the site allowed our crawler in robots.txt. |
| `sagescore.org/` (homepage) | Single input, one CTA: "Audit any domain". |
| `sagescore.org/about` | Methodology, scoring weights, who runs it, contact. |
| `sagescore.org/privacy` and `/remove` | GDPR-mandatory privacy policy + 1-form removal request (auto-`noindex`s the page within 24h, hard-deletes within 30 days). |
| `sagescore.org/api/v1/score?domain=` (later) | JSON endpoint, rate-limited, free tier 5/day, paid tier via API key. |

**Audit dimensions (the 0–100 score):** weighted average of six sub-scores, each 0–100.

| Sub-score | Weight | What it measures (no API calls, pure HTML/HTTP) |
|---|---|---|
| Structured data | 25% | Presence/validity of `Article`, `FAQPage`, `HowTo`, `Product`, `Organization` JSON-LD. Schema.org is the #1 AEO signal. |
| AI-crawler access | 20% | `robots.txt` rules for `GPTBot`, `PerplexityBot`, `ClaudeBot`, `Google-Extended`. Presence of `llms.txt`. |
| Content structure | 20% | Heading hierarchy (`h1`–`h6` order), paragraph length, BLUF density (does the first sentence answer the question?), readability. |
| Entity clarity | 15% | Author/Org markup, About page presence, NAP consistency, social profiles linked. |
| Technical SEO baseline | 10% | Canonical tag, meta description, title length, sitemap, OG/Twitter cards. |
| Citation-worthiness signals | 10% | Outbound citations to authoritative domains, internal anchor density, freshness signal (`dateModified`). |

⚠️ **Explicitly out of scope for v0.1** — anything requiring an LLM API call, anything requiring rendered JavaScript (use plain HTTP `GET` only), backlink data, traffic estimates, paid-tool integrations.

## 5. Out of scope for v0.1 (parked)

- Time-series tracking ("how has my score changed?") — paid feature for later.
- Page-level audits (only domain-level for v0.1; sample 5 representative URLs from sitemap).
- Comparison pages (`sagescore.org/example.com-vs-example2.com`).
- Headless browser rendering (Playwright/Puppeteer).
- Real LLM citation tracking (the crowded category — stay out of it).

## 6. Constraints & non-negotiables

| Constraint | Rule |
|---|---|
| **GDPR / privacy** | (a) No personal data collected from audited sites — facts about HTML only. (b) Privacy policy live before first public audit. (c) Self-serve `/remove` form. (d) 30-day hard-delete SLA. (e) Cookie banner only if analytics. (f) `noindex` available per-domain on owner request. |
| **Robots.txt compliance** | Strict. If a domain disallows `SageScoreBot` user-agent, the public page returns "Owner has opted out" instead of an audit. This is *also* a marketing message: "respectful crawler" is a brand asset. |
| **Rate limiting** | Single-domain crawl never exceeds 1 request every 5 seconds, max 10 pages per domain (sitemap-sampled). |
| **Identifiable user agent** | `SageScoreBot/0.1 (+https://sagescore.org/bot; contact@sagescore.org)` — this is the textbook compliance pattern. |
| **Cache & re-crawl** | Each audit cached 30 days. Owner can request re-audit via signed link from the audit page (light email verification, no account). |
| **Score methodology page** | Full transparent weights, no "secret sauce". This is also a defamation defence — the score is reproducible, not editorial. |
| **Defensive language** | Audit pages say *"based on public HTML signals"*, not *"this site is bad at AEO"*. The score describes the page, never the business. |

## 7. Tech stack

| Layer | Choice | Rationale |
|---|---|---|
| Audit engine | Go binary (the same one that powers your CLI from the previous discussion) | Single codebase = CLI + web service. Reuses goroutines for concurrent crawling. |
| Web server | Go `net/http` + `chi` router | Single static binary, no Node, no PHP. Deploys anywhere. |
| Templating | Go `html/template` server-rendered | Indexable by default, no React hydration needed, fast. |
| Cache / store | SQLite (v0.1) → Postgres later | One file. Free hosting. Migrate when traffic justifies. |
| Hosting | Hetzner / Fly.io single VPS | €5–€15/month. Costs scale with success, not with launch. |
| Crawler | `colly` (Go) with custom user-agent and rate-limiter | Industry standard, robots.txt aware out of the box. |
| Analytics | Plausible (self-hosted, EU) | Privacy-respecting, no cookie banner needed. |

## 8. Naming & branding decision

✏️ **"AEO / GEO / AIO" → just "AI-Search Readiness".** Use one user-facing term across the product and SEO meta titles. Mention AEO and GEO once in the methodology page for keyword coverage. Don't headline three acronyms — it reads like a content farm.

The product is **SageScore**. The page heading is *"AI-Search Readiness Audit"*. The score label is *"SageScore"* (e.g. *"yoursite.com scored 67 SageScore"*). One brand, one number.

## 9. Success metrics (v0.1, first 90 days post-launch)

| Metric | Target |
|---|---|
| Indexed audit pages in Google | 1,000+ |
| Organic visits to `sagescore.org` | 500/month by month 3 |
| Email signups attributed to SageScore traffic | 50/month by month 3 |
| Removal requests | < 2% of audits — higher means UX is too aggressive |
| Cost (hosting + domain) | < €30/month |
| Founder hours sunk | < 80h total to v0.1 ship |

## 10. Risks & mitigations

| Risk | Mitigation |
|---|---|
| Google deindexes for thin pSEO content | Each page must have ≥800 words of *unique* per-domain content (the actual audit findings, not boilerplate). Add a "what is AEO" section that varies by detected CMS. |
| Domain owners get angry | Public, instant, no-account removal form. Polite language. The `/remove` page is the fastest 1-form on the site. |
| Defamation claims | Score methodology fully public; describe the page, not the business; never use evaluative language ("bad", "broken", "neglected"). |
| GDPR investigation | Privacy policy live before public launch. Legitimate-interest assessment documented internally. Personal data not collected. |
| Server overload from viral traffic | SQLite cache + Cloudflare in front. Cap concurrent crawls at 50. |
| Competitors copy | First-mover speed, plus the upsell into SAGE GRIDS plugins is the moat — copying SageScore alone gets nothing. |

## 11. Open questions to decide before any code is written

1. Audit a domain only if visitor explicitly requests it, or pre-generate from a seed list of 5,000 domains? *(Recommended: visitor-triggered + a small founder-curated seed list of 200 well-known sites for SEO bait.)*
2. Should the audit page include the audited site's own logo/favicon? *(Recommended: yes for favicon, no for logo — favicon is fair use, logo is trademark.)*
3. Free API tier or no API in v0.1? *(Recommended: no API in v0.1 — adds support burden.)*
4. Do we publish the underlying Go scorer as the open-source `SageScore` repo we already discussed? *(Strongly recommended: yes. The OSS repo is the proof-of-methodology defence.)*

## 12. Phased delivery

| Phase | Deliverable | Window |
|---|---|---|
| 0 — Decisions | Brief approved, methodology locked, privacy policy drafted, `/remove` flow specified | 2 days, post-launch |
| 1 — Engine | Go scorer producing JSON output for any URL | 1 week |
| 2 — Web service | `sagescore.org/<domain>` live, single audit on demand | 1 week |
| 3 — pSEO fitness | Sitemap, schema markup, internal linking, methodology page, privacy policy, removal flow | 1 week |
| 4 — Soft launch | Submit to Google Search Console, manually generate 200 seed audits, post on Hacker News with the methodology angle | 3 days |
| **Total** | | ~24 working days, June 2026 |

---