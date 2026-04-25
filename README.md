# SageScore

**AI-Search Readiness Audit, free and public.**

`sagescore.org/<domain>` is a 0–100 score plus prioritised fixes for any website's AEO/GEO/AIO readiness — generated entirely from public on-page signals. No LLM API calls, no JavaScript rendering, no scraping of personal data.

The scoring engine is open source. The methodology is fully documented and reproducible.

---

## Status

**Phase 0 — Decisions & Scaffold.** The repo is bootstrapping. The CLI and web service are stubs. See [`tasks/index.md`](tasks/index.md) for the full implementation roadmap.

| Phase | Status |
|---|---|
| 0 — Decisions & scaffold | In progress (Apr 2026) |
| 1 — Scoring engine | Pending (May 19–25) |
| 2 — Web service | Pending |
| 3 — pSEO fitness, removal flow | Pending |
| 4 — Soft launch | Pending (target: Jun 30, 2026) |

---

## Documents

- [`docs/PRD.md`](docs/PRD.md) — product brief.
- [`docs/Technical_Plan.md`](docs/Technical_Plan.md) — technical architecture.
- [`docs/methodology.md`](docs/methodology.md) — locked v0.1 scoring weights and algorithm.
- [`docs/decisions.md`](docs/decisions.md) — binding decisions log.
- [`docs/privacy.md`](docs/privacy.md) — privacy policy.
- [`docs/lia.md`](docs/lia.md) — legitimate-interest assessment.
- [`docs/removal-flow.md`](docs/removal-flow.md) — `/remove` flow specification.
- [`tasks/index.md`](tasks/index.md) — phased implementation plan.

---

## Build

Requires Go 1.24+.

```sh
make build      # build the CLI and web binaries
make test       # run the test suite
make vet        # go vet
```

Phase 1 will enable:

```sh
make audit DOMAIN=example.com
```

---

## License

MIT. See [`LICENSE`](LICENSE).
