# SageScore v0.1 — Implementation Plan

**Companion to:** [`docs/PRD.md`](../docs/PRD.md), [`docs/Technical_Plan.md`](../docs/Technical_Plan.md)
**Owner:** Ilyas Serter
**Build window:** 2026-05-15 → 2026-06-30 (~24 working days)
**Total budget:** < 80 founder hours, < €30/month runtime

---

## How to read this plan

The plan is split into 5 phases, each with its own document under `tasks/`. Each phase doc lists concrete, sequenced tasks with acceptance criteria. Work the phases in order — each one assumes the prior one shipped.

Don't move to the next phase until that phase's **exit gate** is green. Phases are intentionally short so that "stuck on something for 2 days" means "stop and re-plan", not "keep grinding".

| Phase | Document | Window | Working days | Exit gate |
|---|---|---|---|---|
| 0 | [phase-0-decisions.md](phase-0-decisions.md) | May 15–18 | 2 | All PRD §11 questions resolved, repo scaffolded, CI green on empty build |
| 1 | [phase-1-engine.md](phase-1-engine.md) | May 19–25 | 5 | `sagescore audit example.com` produces JSON with all 6 sub-scores + ≥3 article PageAudits |
| 2 | [phase-2-web-service.md](phase-2-web-service.md) | May 26–Jun 1 | 5 | `sagescore.org/<domain>` renders a real audit; SQLite cache works; healthz green |
| 3 | [phase-3-pseo-fitness.md](phase-3-pseo-fitness.md) | Jun 2–8 | 5 | All audit pages ≥800 words, sitemap valid, removal flow E2E, content-lint green |
| 4 | [phase-4-soft-launch.md](phase-4-soft-launch.md) | Jun 9–11 | 3 | 200 seed audits live, GSC submitted, Plausible recording, uptime probe green |
| Buffer | — | Jun 12–30 | ~7 | Slippage + dogfood + HN response |

---

## Phase dependency graph

```
[Phase 0: Decisions]
        │
        ▼
[Phase 1: Engine]   ───────────┐
        │                      │
        ▼                      │ (CLI is usable independently)
[Phase 2: Web Service]         │
        │                      │
        ▼                      │
[Phase 3: pSEO Fitness] ◄──────┘
        │
        ▼
[Phase 4: Soft Launch]
```

Phase 1 unblocks both Phase 2 (web service consumes the scorer) and standalone CLI use. Phase 3 needs Phase 2's rendering layer.

---

## Cross-cutting principles

These apply across every phase. Pin them somewhere visible and review at each gate.

1. **No LLM API calls in the audit path.** Anywhere. Ever in v0.1. (PRD §4)
2. **Determinism.** Same domain + same scorer version = same score. No randomness in sampling or scoring. (Technical Plan §3.3)
3. **Defensive language.** No "bad", "broken", "neglected", "lazy" in any user-facing template. CI lint enforces. (Technical Plan §6.4)
4. **Crawler manners.** 1 req / 5s per host, identifiable UA, robots.txt honoured, /bot info page, instant opt-out. (PRD §6, Technical Plan §3.4)
5. **800-word floor.** Audit page is `noindex` until it has ≥800 words of unique content. (PRD §10, Technical Plan §4.4)
6. **Single binary, single SQLite file.** Don't reach for microservices, queues, or Redis. (Technical Plan §1)
7. **Database is GORM-abstracted.** All persistence goes through the `pkg/store` repository interface. SQLite today, Postgres tomorrow, no app-layer changes. (Technical Plan §5)

---

## Definition of done for v0.1

The launch is shippable when **all** of these are true:

- [ ] `sagescore.org/<domain>` returns a complete server-rendered audit for any valid public domain in <10s on a warm cache, <60s on a cold one.
- [ ] All 6 sub-scores + per-page scores are computed for every audit; ≥3 article pages where discoverable.
- [ ] Privacy policy, methodology, /about, /remove pages live; LIA on file internally.
- [ ] Removal flow works end-to-end: submit → email → confirm → noindex → 30-day reaper deletes content.
- [ ] Sitemap includes ≥200 seed audits; submitted to Google Search Console.
- [ ] Plausible recording, healthz probed, alerts wired.
- [ ] OSS `pkg/scorer` repo published with README and methodology link.
- [ ] HN post drafted (do not post until soak day).
- [ ] Total monthly cost projection < €30.

---

## Out of scope reminder

Anything in PRD §5 plus: no JS rendering, no LLM calls, no public API, no accounts, no comparison pages, no time-series. If a task starts to drift into one of these, **stop and re-scope**.

---

## Tracking

This file plus the per-phase docs *are* the tracking system. As tasks complete, check them off in their phase doc. The index stays static; phase docs evolve.

If you're reading this from the future and the plan diverged from reality, trust git history over the docs.
