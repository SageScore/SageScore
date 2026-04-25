# Phase 3 — pSEO Fitness, Privacy, Removal Flow

**Window:** 2026-06-02 → 2026-06-08 (5 working days)
**Goal:** Make every audit page indexable, defensible, and removable. This is the phase that turns a "tool" into a "GDPR-compliant pSEO asset".
**Exit gate:** All audit pages ≥800 words; sitemap valid and submittable; removal flow E2E green; content-lint blocks defamatory language; methodology and privacy pages live.

---

## Why this phase exists

Phase 2 made the audit page render. This phase makes Google index it, owners able to remove it, and lawyers unable to argue it's defamatory. Skipping any of these turns the launch into a liability.

---

## Sequencing

```
Sitemap + schema + indexability gate → CMS-variant content (800-word floor) → /about, /methodology, /privacy → /remove flow → content lint
```

The CMS-variant content step is the longest writing task — front-load it Day 2 so the rest of the week has slack.

---

## Day-by-day plan

### Day 1 — Sitemap + schema on our pages + indexability gate

- [ ] `pkg/web/handlers/sitemap.go`:
  - Generate `/sitemap.xml` from `AuditRepo.ListSitemap`, filtered to rows where `noindex=false` AND `opt_out=false` AND `word_count >= 800`.
  - Cap each sitemap file at 50,000 URLs; emit a sitemap-index when over cap.
  - `Cache-Control: public, max-age=3600`.
  - Last-mod = `audits.fetched_at`.
- [ ] `templates/audit.gohtml` schema enrichment:
  - Embed `<script type="application/ld+json">` with `WebPage`, `Dataset` (the audit), `BreadcrumbList`, `Organization` (SageScore itself).
  - Canonical: `https://sagescore.org/<lowercased-host>` always.
- [ ] Indexability gate (Technical Plan §4.4) implemented as a render-time check:
  - All-of: SageScoreBot allowed in audited site's robots.txt at fetch time AND word count ≥ 800 AND no active removal AND audit completed AND no domain-level `noindex` flag → `index,follow`.
  - Otherwise → `noindex,nofollow`.
- [ ] Word-count check at render time: count rendered visible text (strip HTML), compare to 800. If under, audit page is rendered but with `noindex` AND a banner: "this audit didn't generate enough unique content to publish — re-audit when this site has more public content".
- [ ] Test: 5 fixtures (passes all gates, fails on each gate independently).

### Day 2 — CMS-variant content (the writing day)

- [ ] `content/cms-wordpress.md`, `cms-shopify.md`, `cms-webflow.md`, `cms-squarespace.md`, `cms-nextjs.md`, `cms-generic.md` — each ~250 words explaining "what is AEO and what does this score mean for a [CMS] site". Specific actionable advice per CMS.
- [ ] `content/findings/*.md` — one per finding code, ~50–100 words explaining what the finding means, why it matters for AEO, evidence placeholder, and a CTA to a SAGE GRIDS plugin where applicable. Cover all finding codes from Phase 1.
- [ ] `pkg/render/content.go`:
  - Loads markdown fragments at startup, caches parsed templates.
  - Selects CMS variant via the `Audit.CMS` field.
- [ ] Verify rendered audit page hits ≥800 words on the 5 staging-test domains.

### Day 3 — Static pages: /about, /methodology, /privacy, /bot

- [ ] `templates/about.gohtml` — who runs SageScore, contact email, OSS link, the methodology promise.
- [ ] `templates/methodology.gohtml` — full transparent weights table, scoring algorithm explainer, version policy, OSS scorer link with a permalink to the version that produced the audit. Must match `docs/methodology.md` content.
- [ ] `templates/privacy.gohtml` — render `docs/privacy.md` via markdown, plus a contact form/mailto.
- [ ] `templates/bot.gohtml` — `/bot` page describing the SageScoreBot UA, what it fetches, rate limits, robots.txt support, opt-out instructions. Linked from the UA string.
- [ ] `static/robots.txt` — allow everything; disallow `/remove`, `/recrawl/*`.
- [ ] All static pages: hit them in browser, confirm rendering, links, and that they're in nav.

### Day 4 — Removal flow end-to-end

- [ ] `pkg/web/handlers/remove.go`:
  - `GET /remove` — render form with email field. Domain optionally pre-filled from `?domain=`.
  - `POST /remove`:
    - Validate email format.
    - Check rate limit (5/IP/day).
    - Insert `RemovalRequest` row.
    - **Immediately** flip `audits.noindex = 1` and purge Cloudflare cache for `/<domain>` (Technical Plan §6.1 step 2).
    - Email a verification token via Resend (using `pkg/email/resend.go`).
    - Render "removal pending" page with `Cache-Control: no-store`.
  - `GET /remove/confirm?token=...`:
    - Look up token, mark `confirmed_at = now()`.
    - Render "confirmed" page; the audit row is now permanently `noindex`.
- [ ] `pkg/email/resend.go` — thin Resend client. Templated HTML email with verification link.
- [ ] `cmd/sagescore-web --reap` — nightly job (run via systemd timer) that:
  - Finds `RemovalRequest` rows where `confirmed_at` is older than 30 days AND `deleted_at IS NULL`.
  - Hard-deletes the matching `Audit` row (cascades to `PageAudit`).
  - Sets `RemovalRequest.deleted_at = now()`.
  - Logs counts.
- [ ] `templates/remove_form.gohtml`, `templates/remove_pending.gohtml`, `templates/remove_confirmed.gohtml`.
- [ ] E2E test: submit form against staging → email received → click link → audit page returns `noindex` → wait simulated 30 days → reaper deletes → audit page 404s.

### Day 5 — Content lint, internal linking, polish

- [ ] `scripts/lint-content.go`:
  - Walks `templates/` and `content/`.
  - Fails if any template/fragment contains "bad", "broken", "neglected", "lazy", "incompetent", "outdated", "garbage", "terrible" (case-insensitive, word-boundary).
  - Wired into CI.
- [ ] Internal linking on audit pages (Technical Plan §7):
  - Pick 5 sibling audits via `AuditRepo.ListSimilar(domain, scoreBand, tld, randomSeed)`.
  - Render in a "Compare with similar sites" block at the bottom.
- [ ] OpenGraph image polish: confirm score number, favicon, domain name all render legibly at 1200×630.
- [ ] Plausible self-hosted: install on the same VPS (Docker container, share the Caddy reverse proxy, no separate domain).
- [ ] Anti-abuse polish on `/remove`:
  - Email-domain matching: prefer emails from `@<domain>` or common admin variants. If not, the request still works but the confirmation email subject line says "manual review may be needed" — for v0.1, manual review is just a log line we'll watch.
- [ ] Final pre-launch audit (against ourselves): does `sagescore.org/sagescore.org` itself produce a score ≥ 80? If not, fix our own page.

---

## Acceptance criteria

- [ ] All audit pages: ≥800 words OR `noindex`. Verified on 10 staging domains.
- [ ] `/sitemap.xml` valid (passes Google's sitemap validator).
- [ ] All 6 finding-code categories have markdown explainers.
- [ ] `/about`, `/methodology`, `/privacy`, `/bot`, `/remove` all live and reachable.
- [ ] Removal flow E2E green on staging including reaper job.
- [ ] Content lint green; CI fails on intentional violation.
- [ ] Sibling-link block renders on every audit page with 5 links.
- [ ] Plausible recording on staging.
- [ ] We pass our own audit at ≥ 80.

---

## Risks specific to this phase

| Risk | Mitigation |
|---|---|
| 800-word floor pushes us into thin-paraphrase territory that Google still penalises | The CMS-variant copy is the differentiator. Each variant is genuinely different advice; not Spintax. |
| Removal email goes to spam, owner gets angry | Use a real sender domain with SPF/DKIM/DMARC set up via Resend. Plain-text fallback in the email. |
| Resend API outage on a removal | The `noindex` flag is set *before* the email is sent. Even if email fails, the page is already de-listed. The removal completes when the user retries. |
| Reaper deletes something it shouldn't | Reaper job logs every deletion; runs in dry-run mode for the first week (logs + counts but no actual delete). Promote to live after sanity-check. |
| Defamation language sneaks in via a finding markdown fragment | Content lint runs on `content/findings/*.md` too, not just templates. |
| Sibling-link block leaks `noindex` audits | Filter at query time: only `noindex=false AND word_count >= 800`. |

---

## Deliverables checklist

- `pkg/web/handlers/sitemap.go`, `pkg/web/handlers/remove.go`.
- `pkg/email/resend.go`.
- `pkg/render/content.go` plus 6 CMS variants and ~30 finding fragments.
- `templates/about.gohtml`, `methodology.gohtml`, `privacy.gohtml`, `bot.gohtml`, `remove_*.gohtml`.
- `scripts/lint-content.go` wired into CI.
- Reaper job + systemd timer.
- Plausible installed.

---

## Estimated effort

- Day 1: 6h
- Day 2: 8h (writing day — biggest single chunk)
- Day 3: 6h
- Day 4: 8h (removal flow + email + reaper is a lot of moving parts)
- Day 5: 6h

**Total: ~34h, full week.**
