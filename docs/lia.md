# Legitimate-Interest Assessment (LIA) — SageScore Audit Publishing

**Document type:** Internal record under UK / EU GDPR Art. 6(1)(f).
**Owner:** Ilyas Serter, Controller.
**Date:** 2026-04-25.
**Review schedule:** Annually, or upon material change in scope (e.g. moving from public HTML to scraped user content — which we explicitly do not do in v0.1).

This LIA documents our reasoning for relying on **legitimate interest** as the lawful basis for fetching publicly available HTML signals from third-party websites and publishing per-domain audit pages on `sagescore.org`. It follows the ICO's three-part test: **purpose**, **necessity**, **balancing**.

---

## 1. Purpose test

> What is the legitimate interest? Why is the processing important to us?

**The interest:** Making AI-search readiness (AEO/GEO) signals visible and comparable across the public web, so that site owners, marketers, and the broader ecosystem can understand and improve their preparedness for citation by AI-search systems.

**Why it matters:**
- Site owners largely lack diagnostic tools for how their sites appear to AI-search crawlers.
- The signals we measure (schema.org markup, robots.txt rules for AI crawlers, content structure) are already public — the value is in aggregation, scoring, and explanation.
- A transparent, methodology-public scorer materially improves industry literacy on AEO topics.
- Commercial benefit to the operator: SageScore funnels into the SAGE GRIDS plugin business (PRD §2 job 3), which is a legitimate business interest.

**Specific, real, present:** ✓ All three. AI-search systems are in active deployment; the diagnostic gap is real today.

---

## 2. Necessity test

> Is the processing necessary to achieve the purpose? Could we achieve the same outcome with less data?

**Necessary?** Yes. To produce a per-domain readiness score we must fetch public HTML, parse it, and compute the score. There is no aggregated-public-data substitute that captures site-specific signals like the canonical tag's accuracy or the validity of a particular `Article` JSON-LD block.

**Could we do less?**
- We could limit ourselves to homepage-only audits — but the methodology requires article pages to assess content structure for the relevant content type (PRD §3, Technical Plan §3). We sample at most 10 pages per domain — significantly fewer than a typical search-engine crawler.
- We could refrain from publishing the audit page and only return scores via API — but public, indexable pages are central to the product's discovery function. Publishing them with strict opt-out makes the same data discoverable to the audited site's owner via Google.

**Conclusion:** The processing is necessary and proportionate. We deliberately do not collect personal data, do not run JavaScript (which can expose more than HTML alone), and do not fetch content behind authentication.

---

## 3. Balancing test

> Do the rights and interests of the data subject override the legitimate interest?

### 3.1 Whose data?

We process **non-personal data about websites** — facts about HTML, headers, schema markup, etc. The data describes the site's structural readiness, not any individual person.

Edge cases where personal data could incidentally be present:
- A homepage HTML that includes the founder's name in an `<h1>`. We do not extract or retain this as personal data; we count words.
- A `<meta name="author" content="Jane Doe">` tag. We assert presence/absence of the tag for the Entity Clarity score; we do not retain "Jane Doe" as a profile-shaped record.
- An `Organization` JSON-LD block with a phone number. We assert NAP completeness; we do not enrich, share, or query against this.

We retain the raw HTML in cache for 30 days for re-rendering and re-scoring purposes. After 30 days it is re-fetched (overwriting) or purged (if owner removed).

### 3.2 Reasonable expectations

A reasonable webmaster expects their public HTML, robots.txt, and sitemap to be fetched by:
- Search engines.
- AI-search crawlers (GPTBot, ClaudeBot, etc.).
- SEO-tooling vendors (Ahrefs, Semrush, etc.).

SageScore's bot operates under the same well-established norms: identifiable user-agent, robots.txt-compliant, rate-limited (1 req / 5s), small footprint (≤10 pages, ≤5MB per response). The audit page itself is a derivative public-interest commentary on already-public signals — analogous in kind to existing third-party SEO audit reports, but free and methodologically transparent.

### 3.3 Mitigations and safeguards

We have implemented multiple safeguards above the GDPR baseline:

| Safeguard | Implementation |
|---|---|
| **Crawler manners** | Identifiable UA `SageScoreBot/0.1 (+https://sagescore.org/bot; contact@sagescore.org)`. 1 req / 5s per host. Hard caps: 10 pages, 5MB/response, 120s/audit. |
| **Robots.txt compliance** | If `SageScoreBot` is disallowed, the public page returns "Owner has opted out" instead of an audit. |
| **Self-serve removal** | One-form, no account required, takes effect within seconds via `noindex` and Cloudflare cache purge. |
| **Hard delete SLA** | 30 days after confirmed removal, audit content is permanently deleted. |
| **Methodology transparency** | Full weights and algorithm published; OSS scorer reproduces every score deterministically. |
| **Defensive language** | Audit pages describe the page's HTML, never the business. CI lints for evaluative language ("bad", "broken", "neglected"). |
| **No personal-data ingest** | We do not extract or build profiles from HTML personal-name occurrences. |
| **No JavaScript execution** | Reduces attack surface and the risk of fetching content the operator did not intend to be public. |

### 3.4 Interests of data subjects (site owners)

Site owners' interests:
- **Reputation:** Mitigated by defensive-language linting and the methodological framing of the score as a description of HTML signals.
- **Privacy:** No personal data of the owner is collected.
- **Control:** Self-serve `/remove` and re-audit links provide direct control without account creation.
- **Server load:** Conservative crawl footprint is well within normal-traffic thresholds.

We track removal requests as a percentage of audits as a key metric (PRD §9: target <2%). A material rise would trigger a reassessment of this LIA.

---

## 4. Conclusion

The processing meets all three parts of the legitimate-interest test:

1. **Purpose:** Real, specific, public-interest, with a documented commercial component.
2. **Necessity:** The processing is necessary, minimal, and uses only public signals.
3. **Balancing:** Site owners' rights and interests are protected via comprehensive safeguards — many above GDPR baseline. We do not process personal data of third parties.

We rely on Art. 6(1)(f) (legitimate interest) for the audit-publishing function, and on Art. 6(1)(c) (legal obligation) plus Art. 6(1)(f) (legitimate interest) for the removal-flow data we collect from owners requesting erasure.

---

## 5. Triggers for re-assessment

This LIA must be re-assessed if any of the following change:

- We begin extracting personal data from third-party sites.
- We begin processing more than public HTML (e.g. running JavaScript, fetching authenticated content, third-party API enrichment).
- Removal-request rate exceeds 2% of audits.
- We receive a data-protection authority enquiry or complaint.
- We move infrastructure outside the EEA.
- We add a public API or any feature that materially expands processing scope.

A failed re-assessment requires either narrowing the processing or migrating to a different lawful basis (likely consent, which would necessitate a major UX redesign).
