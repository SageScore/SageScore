# Phase 2 — Web Service

**Window:** 2026-05-26 → 2026-06-01 (5 working days)
**Goal:** `sagescore.org/<domain>` serves a live, server-rendered audit, backed by a SQLite cache via GORM, with the audit engine from Phase 1 as the compute layer.
**Exit gate:** `make run-web` boots; visiting `localhost:8080/example.com` runs an audit, caches it, and renders an HTML page with all sub-scores + per-page breakdown; `/healthz` returns 200.

---

## Why this phase exists

This is where the scorer becomes a product. Most of the work is plumbing: HTTP routing, database access, templating, and a job queue that doesn't melt under concurrent traffic. The pSEO polish comes in Phase 3 — here we just need the surface to *work*.

---

## Sequencing

```
GORM store + models → chi router + handlers → audit submit/fetch flow → templates → OG image → caching → deploy scaffold
```

Each step verifies independently. Don't write the next layer until the previous one has at least one happy-path test.

---

## Day-by-day plan

### Day 1 — Store layer (GORM)

- [ ] `pkg/store/db.go`:
  - `Open(driver, dsn) (*gorm.DB, error)` — switch on driver: `"sqlite"` → `gorm.io/driver/sqlite`, `"postgres"` → `gorm.io/driver/postgres`.
  - WAL mode + busy timeout pragmas applied for SQLite.
  - `AutoMigrate` on all models at startup.
- [ ] `pkg/store/models.go`:
  - `Audit`, `PageAudit`, `RemovalRequest`, `RecrawlToken`, `RateLimit` exactly as in Technical Plan §5.2.
  - 1:N association from `Audit` to `PageAudit` with cascade delete.
- [ ] `pkg/store/repo.go`:
  - Interface `AuditRepo` with `Get(domain) (*Audit, bool, error)`, `Upsert(a *Audit) error`, `MarkNoindex(domain) error`, `Delete(domain) error`, `ListSitemap(limit, offset) ([]Audit, error)`.
  - Interface `RemovalRepo` with `Create`, `ConfirmByToken`, `ListPendingDeletes`, `HardDelete`.
  - Interface `RateLimitRepo` with `Check(ipHash, bucket, window, limit) (allowed bool, error)`.
  - GORM-backed implementations in `pkg/store/gorm_*.go`.
- [ ] Repository tests with in-memory SQLite (`sqlite.Open(":memory:")`).
- [ ] `make migrate` target for explicit schema apply (uses AutoMigrate today, will switch to goose when first non-trivial migration lands).

**End of day:** `go test ./pkg/store/...` green; `Audit` records round-trip with their `PageAudit` children.

### Day 2 — HTTP router, middleware, audit submit flow

- [ ] `pkg/web/router.go`:
  - `chi` router. All routes from Technical Plan §4.1.
  - Middleware stack: request ID → structured logger (`slog`) → recover → real-IP → CORS-disable → timeout (60s default).
- [ ] `pkg/web/middleware/ratelimit.go`:
  - Per-IP-hash limiter using `RateLimitRepo`. Buckets: `audit` (10/hour), `remove` (5/day), default (100/min).
  - Returns 429 with `Retry-After`.
- [ ] `pkg/web/handlers/home.go`, `pkg/web/handlers/submit.go`:
  - Home renders the input form + 3 sample audit links from the seed list.
  - Submit validates the domain (parse → lowercase host → reject IPs/own domain/blocklist), 302s to `/<domain>`.
- [ ] `pkg/web/handlers/audit.go`:
  - `GET /{domain}` — cache-check via `AuditRepo.Get`. Three branches:
    - **Fresh (<30 days):** render immediately.
    - **Stale (30–60 days):** render cached + enqueue background re-audit.
    - **Missing:** acquire semaphore slot (size 50), run audit synchronously if slot available within 2s; otherwise queue + render "computing" interstitial with auto-refresh meta tag.
- [ ] `pkg/web/queue.go`:
  - In-process audit queue: bounded channel + worker pool of 10.
  - `Enqueue(domain)` is idempotent (dedupes on already-queued + already-running).
- [ ] Smoke test: `go run ./cmd/sagescore-web`, `curl localhost:8080/example.com` returns plaintext stub with the score.

### Day 3 — Templates and rendering

- [ ] `templates/base.gohtml` — layout with `<head>`, nav, footer, slot for content.
- [ ] `templates/home.gohtml` — single input + 3 sample links + "what is AEO" teaser.
- [ ] `templates/audit.gohtml` — the main audit page:
  - Hero with score, domain, fetched-at, scorer version.
  - Score breakdown table: 6 dimensions with their scores and weights.
  - **Per-page breakdown table**: every sampled URL, kind, score, top finding (Technical Plan §4.3).
  - Per-dimension findings prose (loops over `Finding` list, maps `Code` → markdown template).
  - "What is AEO" teaser block (CMS-variant insertion happens in Phase 3).
  - Methodology footer link.
  - Removal/recrawl link block.
- [ ] `templates/computing.gohtml` — interstitial with auto-refresh meta.
- [ ] `templates/optout.gohtml` — "Owner has opted out" page.
- [ ] `pkg/render/funcs.go` — template funcs: `scoreColor` (red/amber/green band), `prettyDate`, `sentenceCase`, `markdownify` (uses `gomarkdown/markdown`).
- [ ] `pkg/render/findings.go` — `RenderFinding(f Finding) template.HTML` that maps `Code` → markdown fragment under `content/findings/<code>.md`. Each fragment uses `{{.Evidence}}` substitutions.
- [ ] Manual visual review: open 5 different domains in a browser, confirm rendering doesn't break on edge cases (missing data, very long titles, non-Latin text).

### Day 4 — OG image, healthz, caching headers, recrawl flow

- [ ] `pkg/render/ogimage.go`:
  - At audit-write time, render an SVG template substituting score + favicon + domain.
  - Convert to PNG via `github.com/srwiley/oksvg` + `image/png` (no external dependencies, no headless browser).
  - Write to `static/og/<domain>.png`.
  - Linked from the audit page's `<meta property="og:image">`.
- [ ] `pkg/web/handlers/health.go` — `/healthz` returns `{"ok": true, "version": "...", "db": "ok"}` after a `SELECT 1`.
- [ ] Caching headers per Technical Plan §4.2:
  - Fresh audit page: `Cache-Control: public, max-age=86400, stale-while-revalidate=604800`.
  - `/remove`, `/audit` POST, `/recrawl/*`: `Cache-Control: no-store`.
- [ ] `pkg/web/handlers/recrawl.go` — `GET /recrawl/{token}`:
  - Validate token via `RecrawlTokenRepo`, expire-on-use, run synchronous re-audit, redirect to `/{domain}`.
  - Re-audit purges Cloudflare for the domain URL via the Cloudflare API (env-var-configured token).
- [ ] Recrawl link rendered on every audit page, signed with HMAC of `(domain + secret + 30-day expiry)` so we don't have to write a token row for every audit page view.

### Day 5 — Integration testing and deployment scaffold

- [ ] `make e2e` — boots the binary against a tiny in-repo HTTP server (the synthetic-site fixture), runs an audit via HTTP, asserts:
  - Returned HTML contains expected score in expected band.
  - Has `<meta name="robots">` with the right value.
  - Per-page breakdown table contains ≥3 articles.
  - SQLite row written.
- [ ] `deploy/Dockerfile` — multi-stage Go build, distroless final, ~30MB image.
- [ ] `deploy/Caddyfile` — TLS termination, reverse proxy to Go on `:8080`.
- [ ] `deploy/sagescore.service` — systemd unit for the binary.
- [ ] `deploy/litestream.yml` — SQLite replication to Hetzner Object Storage.
- [ ] `.github/workflows/deploy.yml` — on push to `main`: build image, push to GHCR, ssh deploy script pulls + restarts. Manual trigger only for v0.1; auto-deploy after Phase 4.
- [ ] First deploy to Hetzner staging VPS (or Fly.io). Smoke-test `https://staging.sagescore.org/example.com`.

---

## Acceptance criteria

- [ ] `make run-web` boots locally, serves a real audit for any domain.
- [ ] `make e2e` green.
- [ ] `/healthz` returns 200 with DB check.
- [ ] All 6 sub-scores + per-page breakdown render in the audit page.
- [ ] OG image generated and served correctly.
- [ ] Cache hit path: second request for same domain returns in <50ms (cached HTML render).
- [ ] Cold path: first request for new domain returns in <60s including crawl.
- [ ] Concurrent load test: 100 parallel requests against 100 distinct cached domains complete without errors.
- [ ] Staging deploy reachable on a real domain.

---

## Risks specific to this phase

| Risk | Mitigation |
|---|---|
| Cold audit takes >60s and the user loses interest | The "computing" interstitial with auto-refresh handles this. Don't over-engineer with WebSockets in v0.1. |
| Audit kicked off but the user closes the tab → wasted work | The audit completes and writes to cache anyway. The next visitor benefits. |
| GORM AutoMigrate drift between dev and prod | Keep AutoMigrate for v0.1; switch to goose the moment a column needs to be removed or renamed. Test the migration path on a copy of staging before prod. |
| OG image generation slow on cold path | Generate it asynchronously after the audit row is written. The audit page can render before the image exists; the `<meta>` tag's URL just 404s briefly on social previews — acceptable for v0.1. |
| Cloudflare purge API failure on recrawl | Log + continue. The cached page still has the new content after CF's max-age expires; the user's expectation is that re-audit "shows new score eventually", not "instantly globally". |
| One slow domain blocks the worker pool | 90s hard timeout per audit (Technical Plan §3.4). Worker exits, slot freed. |

---

## Deliverables checklist

- `pkg/store/` complete with GORM models, repo interfaces, repo impls, tests.
- `pkg/web/` complete with router, middleware, handlers, queue.
- `pkg/render/` with templates, finding renderer, OG image generator.
- `templates/` with base, home, audit, computing, optout.
- `deploy/` with Dockerfile, Caddyfile, systemd unit, litestream config.
- `make e2e` target green.
- Staging deploy live.

---

## Estimated effort

- Day 1: 6h (GORM is fast to set up)
- Day 2: 7h (queue + semaphore correctness takes care)
- Day 3: 8h (templates always take longer than expected)
- Day 4: 6h
- Day 5: 7h (deployment is finicky; budget for surprises)

**Total: ~34h, full week.**
