# Phase 0 — Decisions & Scaffold

**Window:** 2026-05-15 → 2026-05-18 (2 working days)
**Goal:** Resolve every blocking decision and stand up an empty-but-CI-green repo, so Phase 1 is purely engineering.
**Exit gate:** All PRD §11 open questions answered; repo scaffolded; CI green on `go build ./... && go vet ./... && go test ./...`.

---

## Why this phase exists

The single biggest waste in a 24-day build is a Phase-2 commit that gets reverted because someone realised the legal-interest assessment hadn't been written. Spend two days here so the next 22 are pure execution.

---

## Tasks

### 0.1 Resolve PRD §11 open questions

For each, write the decision into `docs/decisions.md` (one paragraph each):

- [ ] **Q1: Pre-generate from a seed list?** — Recommended: visitor-triggered + 200 founder-curated seeds. Confirm or override.
- [ ] **Q2: Logo/favicon on audit page?** — Recommended: favicon yes, logo no. Confirm.
- [ ] **Q3: Free API tier in v0.1?** — Recommended: no API in v0.1. Confirm.
- [ ] **Q4: Open-source the scorer?** — Recommended: yes, Apache-2.0. Confirm.
- [ ] **Engine Q: Authoritative-domains list source?** — Static list bundled in `pkg/scorer/data/authoritative.txt`. Confirm.
- [ ] **Engine Q: CMS detection method?** — Hand-curated 12-fingerprint set, headers + meta heuristics. Confirm.
- [ ] **Infra Q: Email provider for removal-confirm?** — Resend (EU region). Confirm.

### 0.2 Lock the scoring methodology

- [ ] Write `docs/methodology.md`:
  - 6 dimensions and their weights (25/20/20/15/10/10).
  - Per-dimension input list and scoring formula (lifted from Technical Plan §3.2).
  - Roll-up rules (homepage 2× / articles 1× / domain-level dimensions from single fetches).
  - "What the score is not" section: not editorial, not a verdict on the business, not a Google ranking factor.
  - Versioning policy: scorer version stamped per audit; old audits never silently re-scored.
- [ ] Cross-link from `README.md` and (later) the rendered methodology page.

### 0.3 Draft privacy + LIA + removal spec

- [ ] `docs/privacy.md`: GDPR privacy policy text (controller identity, lawful basis = legitimate interest, data collected = email + IP-hash for removal flow only, retention, rights, contact).
- [ ] `docs/lia.md`: Legitimate-Interest Assessment — purpose test, necessity test, balancing test, with the "we audit only public HTML, no PII from third parties" finding documented.
- [ ] `docs/removal-flow.md`: 1-page flow spec — input fields, email verification, noindex on submit, 30-day hard-delete SLA, abuse rate-limits.

### 0.4 Repo scaffold

- [ ] `go.mod` with module path `github.com/iserter/sagescore` (or whatever the chosen path is).
- [ ] Directory layout per Technical Plan §2.
- [ ] `cmd/sagescore/main.go` and `cmd/sagescore-web/main.go` with `package main` + `func main() {}` and a one-line print so `go run` works.
- [ ] `pkg/scorer/scorer.go` with the public `Audit(domain string) (Result, error)` signature stubbed.
- [ ] `pkg/store/repo.go` with the repository interface stubbed.
- [ ] `LICENSE` updated to Apache-2.0 if Q4 accepted.

### 0.5 CI scaffold

- [ ] `.github/workflows/ci.yml` running on every push and PR:
  - `go build ./...`
  - `go vet ./...`
  - `go test ./... -race -count=1`
  - `gofmt -l .` (fail if any output)
  - `go run ./scripts/lint-content` (placeholder until Phase 3, exits 0)
- [ ] `Makefile` with `make build`, `make test`, `make run-web`, `make audit DOMAIN=...`.
- [ ] Pre-commit hook stub via `.githooks/pre-commit` running `gofmt -l .`.

### 0.6 Authoritative-domains and CMS-fingerprint seed lists

- [ ] `pkg/scorer/data/authoritative.txt` — 200ish well-known authoritative domains (wikipedia.org, gov, edu, major news outlets). Versioned via the file's git history.
- [ ] `pkg/scorer/data/cms-fingerprints.json` — 12 entries, each with `name`, `header_match`, `meta_match`, `path_match`. WordPress, Shopify, Webflow, Squarespace, Wix, Next.js, Gatsby, Hugo, Ghost, Drupal, Joomla, "unknown" fallback.

---

## Acceptance criteria

- All checkboxes above ticked.
- `go build ./... && go vet ./... && go test ./...` exits 0 on a fresh clone.
- CI green on `main`.
- All decision docs committed and reviewed by the founder.

---

## Risks specific to this phase

| Risk | Mitigation |
|---|---|
| Decision paralysis on §11 questions | Default to the recommendation in PRD §11 unless the founder has a strong reason. Don't open new options. |
| Privacy/LIA writing dragging on | Use a published template (ICO has one). The point is to have it on file, not to perfect it. |
| Repo bikeshedding (module path, license header style) | Time-box to 2 hours total. Pick, move on. |

---

## Deliverables checklist

Files that must exist at end of phase:

- `docs/decisions.md`
- `docs/methodology.md`
- `docs/privacy.md`
- `docs/lia.md`
- `docs/removal-flow.md`
- `go.mod`, `Makefile`, `.github/workflows/ci.yml`
- `cmd/sagescore/main.go`, `cmd/sagescore-web/main.go`
- `pkg/scorer/scorer.go` (stub), `pkg/store/repo.go` (stub)
- `pkg/scorer/data/authoritative.txt`, `pkg/scorer/data/cms-fingerprints.json`
- `LICENSE` (Apache-2.0 if confirmed)

---

## Estimated effort

- 0.1: 1.5h
- 0.2: 2h
- 0.3: 4h (longest single task — privacy text)
- 0.4: 2h
- 0.5: 1h
- 0.6: 2h

**Total: ~12.5h, fits in 2 working days.**
