# Changelog

All notable changes to SageScore are documented here. This file is
managed automatically by [release-please](https://github.com/googleapis/release-please-action).
Do not edit new entries by hand — make conventional-commit commits on
`main` and let release-please assemble the next release.

Conventional-commit types recognised in this repo (`release-please-config.json`):

- `feat:` — new user-facing capability → minor bump (pre-1.0: or patch, configured)
- `fix:` — bug fix → patch bump
- `perf:` — performance improvement
- `refactor:` — non-behavioural code change
- `docs:` — documentation change (shown in changelog)
- `revert:` — revert of a prior commit
- `chore:` / `test:` / `build:` / `ci:` — hidden from the public changelog

A `!` after the type (e.g. `feat!: ...`) or a `BREAKING CHANGE:` footer
triggers a major-version bump.

---

## [0.2.1](https://github.com/SageScore/SageScore/compare/sagescore-v0.2.0...sagescore-v0.2.1) (2026-04-25)


### Features

* implement Phase 1 scoring engine and release-please automation ([3b0bc65](https://github.com/SageScore/SageScore/commit/3b0bc6564256bafd75211992fefa86f030f37392))

## 0.2.0 (2026-04-25)

Initial tracked release — Phase 0 (scaffold) and Phase 1 (scoring
engine) are complete. See `tasks/index.md` for the full roadmap and
`docs/methodology.md` for the normative scoring spec.

### Features

- Scoring engine with six dimensions (Structured Data, AI-Crawler
  Access, Content Structure, Entity Clarity, Technical SEO, Evidence &
  Citation Readiness).
- Deterministic crawler with ≥3-article page sampling.
- CLI: `sagescore audit <domain>` produces JSON or a human summary.
- GORM-ready persistence interfaces in `pkg/store` (implementation
  lands in Phase 2).
- GDPR-grade privacy policy, removal-flow spec, and legitimate-interest
  assessment committed before any public audit is produced.

### Documentation

- Normative methodology in `docs/methodology.md` (v0.2.0 weights are
  grounded in the Princeton GEO paper and 2026 AEO benchmarks; see
  `docs/decisions.md` D-11).
- Implementation plan in `tasks/` (five phases, each with its own
  doc).
