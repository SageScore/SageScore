# Phase 4 — Soft Launch

**Window:** 2026-06-09 → 2026-06-11 (3 working days)
**Goal:** Submit to Google Search Console, seed 200 founder-curated audits, set up production observability, and post to Hacker News with a methodology angle.
**Exit gate:** 200 seed audits live + indexed by GSC, Plausible recording prod traffic, uptime probe green, HN post drafted (not necessarily posted yet — pick the right day).

---

## Why this phase exists

A pSEO play with no seed content has nothing for Google to index and nothing for visitors to land on. A launch with no observability flies blind. A HN post written under launch-day pressure reads like a launch-day post.

Front-load these three so launch day itself is anticlimactic.

---

## Sequencing

```
Production deploy + observability → seed-audit script + 200 runs → GSC submission → HN post draft → soak day
```

The 200 seed audits run *after* prod is up so they hit the real database, the real Cloudflare cache, and the real OG image generation pipeline.

---

## Day-by-day plan

### Day 1 — Production deploy + observability

- [ ] DNS: `sagescore.org` and `www.sagescore.org` point to Hetzner VPS via Cloudflare proxied A record.
- [ ] TLS: Caddy provisions Let's Encrypt cert automatically.
- [ ] Production deploy of the binary (image built at end of Phase 3, deployed today after final review).
- [ ] Production env vars set:
  - `SAGESCORE_DB_DRIVER=sqlite`, `SAGESCORE_DB_DSN=/var/sagescore/db.sqlite`.
  - `SAGESCORE_RESEND_KEY=...`.
  - `SAGESCORE_CLOUDFLARE_TOKEN=...`.
  - `SAGESCORE_HMAC_SECRET=...` (32 random bytes).
  - `SAGESCORE_SCORER_VERSION=0.1.0`.
- [ ] `litestream` running, replicating SQLite to Hetzner Object Storage.
- [ ] Plausible self-hosted on the same VPS, behind Caddy at `/_p/`. Tracker script in base template.
- [ ] BetterStack uptime probe on `https://sagescore.org/healthz`, alert to email + SMS.
- [ ] Sentry free tier wired up; ERROR-and-above logs forwarded.
- [ ] Prometheus metrics endpoint exposed on `:9090`, firewalled to localhost only. Scraped by a tiny `node_exporter` + Prometheus on the same box (optional for v0.1 — uptime probe + Sentry is the minimum).
- [ ] Smoke test prod: audit 5 random domains via the production URL. Confirm rendering, OG image, sitemap, headers.

### Day 2 — Seed-audit script + 200 runs

- [ ] `cmd/sagescore-seed/main.go`:
  - Reads a list of domains from `seeds/founder-curated.txt`.
  - Hits `/{domain}` over HTTPS in serial with 1-second pacing (so we behave like a real visitor, not a self-DDoS).
  - Logs success/failure per domain.
- [ ] `seeds/founder-curated.txt` — 200 hand-picked well-known sites. Curation criteria:
  - 50 SaaS landing pages (clear product story, varied AEO maturity).
  - 50 e-commerce sites (Shopify, BigCommerce, custom).
  - 50 content/blog sites (WordPress, Ghost, Substack).
  - 50 corporate/agency sites (likely lower scores → contrast for visitors).
  - All publicly accessible, none controversial, none competitors of SAGE GRIDS that would create awkward outreach optics.
- [ ] Run the seed script against production. Estimated runtime: 200 × ~30s = ~100 minutes. Monitor for crashes, rate-limit warnings, OG image failures.
- [ ] Spot-check 20 random seed audits in browser. Look for:
  - Rendering bugs (encoding issues, layout breaks).
  - Findings that read awkwardly.
  - Score outliers (a famous site scoring 12 might mean a bug, not a finding).
- [ ] If outliers/bugs found, fix and re-seed the affected subset.

### Day 3 — Google Search Console + HN post + soak

- [ ] Google Search Console:
  - Verify ownership of `sagescore.org` via DNS TXT record.
  - Submit `https://sagescore.org/sitemap.xml`.
  - Submit a small handful of seed-audit URLs directly via "URL inspection" → "Request indexing" (forces a faster initial crawl).
- [ ] Bing Webmaster Tools: same drill (free traffic is free).
- [ ] HN post draft (`docs/hn-post.md`):
  - Title: "Show HN: SageScore – an open-source AEO readiness scorer (no LLM calls)"
  - Body angle: methodology-first. Talk about the scoring weights, the OSS scorer repo, the "no LLM calls" stance, the GDPR-clean removal flow. **Do not** lead with traffic ambitions or upsell.
  - First-comment plant: link to the methodology page and the OSS repo.
  - Plan to post on a Tuesday or Wednesday morning EU time (best HN engagement window for a methodology post).
- [ ] **Soak day** (rest of Day 3 and into the buffer):
  - Watch Plausible for organic traffic.
  - Watch Sentry for new error patterns.
  - Watch removal-request count (PRD §9: must stay <2% of audits).
  - Tail logs for unusual user-agents (scrapers, attackers).
  - Don't make code changes during soak unless something is broken in prod. Resist the urge to polish.

---

## Acceptance criteria

- [ ] `https://sagescore.org/<seed-domain>` returns a real audit for all 200 seed domains.
- [ ] Sitemap submitted and accepted in GSC (initial coverage takes days; "submitted" is the gate, not "indexed").
- [ ] Plausible shows non-zero traffic from the seed run.
- [ ] BetterStack probe green for 24 consecutive hours.
- [ ] Sentry has zero unresolved ERROR-level events from production.
- [ ] HN post draft committed in `docs/hn-post.md`, awaiting founder approval to post.
- [ ] Cost meter for the launch month projects to < €30 (PRD §9).

---

## Risks specific to this phase

| Risk | Mitigation |
|---|---|
| One of the seed domains complains | We have the removal flow live (Phase 3). Worst case: they hit `/remove`, get noindexed in seconds. We don't argue. |
| Hacker News floods us with traffic and Cloudflare can't keep up | Cloudflare's free tier handles tens of thousands of QPS for cached pages. Cold audits are gated by the queue (max 50 concurrent; rest get 503 + Retry-After). The semaphore is the load shield. |
| Indexing takes longer than the 90-day metric window | Expected. The PRD §9 metrics are 90-day targets. GSC indexing for a brand-new domain is typically 2–6 weeks for the first wave. |
| HN post lands flat | Acceptable. HN is one channel; cold-outreach (the SG Lead Manager pipeline) is the more reliable top of funnel per PRD §2 job 2. |
| A bug discovered post-launch needs urgent fix | We have a deploy script and CI. The cost is low. **Don't panic-deploy without running the test suite.** |
| Litestream replication silently broken | Day 1 includes a manual restore drill: stop the service, delete `db.sqlite`, run `litestream restore`, restart, confirm an audit is present. Don't skip this. |

---

## Deliverables checklist

- Production VPS live, TLS valid, healthz green.
- 200 seed audits cached and indexed.
- `seeds/founder-curated.txt` committed.
- `cmd/sagescore-seed/main.go`.
- BetterStack + Sentry wired.
- Plausible recording.
- GSC + Bing submitted.
- `docs/hn-post.md` drafted.

---

## Estimated effort

- Day 1: 7h (deploys always have surprises)
- Day 2: 6h (seed run is mostly waiting; spot-check is the work)
- Day 3: 4h active + soak observation

**Total: ~17h across 3 days.**

---

## After this phase

The remaining buffer (June 12–30) is for:

- HN post timing.
- Whatever bug Phase 4 surfaces.
- Drafting cold-outreach copy that uses the audit URL as the payload (handoff to SG Lead Manager).
- v0.2 planning conversation: which of the parked items in PRD §5 graduate first? Time-series tracking? Page-level audits? API tier?

If everything went smoothly, the buffer is also the moment to write the v0.1 retrospective: what took longer than estimated, what to keep, what to kill before v0.2.
